package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/hra42/openrouter-go"
)

// SchedulerConfig holds configuration for the scheduler
type SchedulerConfig struct {
	Schedule         string  // Cron expression or predefined schedule
	DateFilter       string  // Optional date filter
	DBPath           string  // Database path
	ExportPath       string  // Optional export path
	S3Config         *S3Config // Optional S3 configuration
	Verbose          bool    // Verbose logging
	Timezone         string  // Timezone for scheduling (default: UTC)
}

// JobConfig holds configuration for a single job execution
type JobConfig struct {
	Client     *openrouter.Client
	DB         *sql.DB
	DateFilter string
	ExportPath string
	S3Config   *S3Config
	Verbose    bool
}

// RunScheduler starts the scheduler and runs jobs according to the schedule
func RunScheduler(ctx context.Context, config *SchedulerConfig) error {
	log.Printf("Starting OR Analytics Scheduler")
	log.Printf("Schedule: %s", config.Schedule)
	log.Printf("Database: %s", config.DBPath)
	log.Printf("Timezone: %s", config.Timezone)

	if config.ExportPath != "" {
		log.Printf("Export: %s", config.ExportPath)
	}

	// Get API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
	}

	// Create OpenRouter client
	client := openrouter.NewClient(
		openrouter.WithAPIKey(apiKey),
		openrouter.WithReferer("https://github.com/hra42/or-analytics"),
		openrouter.WithAppName("OR Analytics Scheduler"),
	)

	// Connect to DuckDB and initialize table
	db, err := InitDB(config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Load timezone
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		return fmt.Errorf("failed to load timezone %s: %w", config.Timezone, err)
	}

	// Create scheduler
	s, err := gocron.NewScheduler(gocron.WithLocation(location))
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}
	defer func() {
		if err := s.Shutdown(); err != nil {
			log.Printf("Error shutting down scheduler: %v", err)
		}
	}()

	// Job configuration
	jobConfig := &JobConfig{
		Client:     client,
		DB:         db,
		DateFilter: config.DateFilter,
		ExportPath: config.ExportPath,
		S3Config:   config.S3Config,
		Verbose:    config.Verbose,
	}

	// Define the job function
	jobFunc := func() {
		if err := runImportJob(ctx, jobConfig); err != nil {
			log.Printf("Error running scheduled job: %v", err)
		}
	}

	// Schedule the job
	var job gocron.Job
	if config.Schedule == "daily" || config.Schedule == "" {
		// Default: daily at midnight
		job, err = s.NewJob(
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(0, 0, 0))),
			gocron.NewTask(jobFunc),
		)
	} else if config.Schedule == "hourly" {
		job, err = s.NewJob(
			gocron.DurationJob(time.Hour),
			gocron.NewTask(jobFunc),
		)
	} else if config.Schedule == "now" {
		// Run once immediately, then continue with schedule
		if err := runImportJob(ctx, jobConfig); err != nil {
			log.Printf("Error running initial job: %v", err)
		}

		// Then set up daily schedule
		job, err = s.NewJob(
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(0, 0, 0))),
			gocron.NewTask(jobFunc),
		)
	} else {
		// Custom cron expression
		job, err = s.NewJob(
			gocron.CronJob(config.Schedule, false),
			gocron.NewTask(jobFunc),
		)
	}

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	// Log next run time
	nextRun, err := job.NextRun()
	if err == nil {
		log.Printf("Next scheduled run: %s", nextRun.Format(time.RFC3339))
	}

	// Start the scheduler
	s.Start()

	log.Println("Scheduler started. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("Received interrupt signal, shutting down...")
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	return nil
}

// runImportJob executes a single import job
func runImportJob(ctx context.Context, config *JobConfig) error {
	startTime := time.Now()
	log.Printf("Starting import job at %s", startTime.Format(time.RFC3339))

	// Fetch activity data
	var options *openrouter.ActivityOptions
	if config.DateFilter != "" {
		options = &openrouter.ActivityOptions{
			Date: config.DateFilter,
		}
		if config.Verbose {
			log.Printf("Fetching activity data for: %s", config.DateFilter)
		}
	} else {
		if config.Verbose {
			log.Println("Fetching all activity data (last 30 completed UTC days)")
		}
	}

	activity, err := config.Client.GetActivity(ctx, options)
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}

	if config.Verbose {
		log.Printf("Retrieved %d activity records", len(activity.Data))
	}

	if len(activity.Data) == 0 {
		log.Println("No activity data found")
		return nil
	}

	// Convert and insert records
	records := ConvertActivityData(activity.Data)
	inserted := 0

	// Insert in batches
	for i := 0; i < len(records); i += 100 {
		end := i + 100
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		count, err := InsertActivityRecords(config.DB, batch)
		inserted += count

		if err != nil {
			log.Printf("Warning: error inserting batch: %v", err)
		}

		if config.Verbose && inserted%100 == 0 {
			log.Printf("  Processed %d records...", inserted)
		}
	}

	log.Printf("Successfully imported %d records", inserted)

	// Export to Parquet if requested
	if config.ExportPath != "" {
		if IsS3Path(config.ExportPath) {
			if config.Verbose {
				log.Printf("Exporting data to S3: %s", config.ExportPath)
			}
			if err := ExportToS3(ctx, config.DB, config.ExportPath, config.S3Config); err != nil {
				return fmt.Errorf("failed to export to S3: %w", err)
			}
			log.Printf("Successfully exported to %s", config.ExportPath)
		} else {
			if config.Verbose {
				log.Printf("Exporting data to Parquet file: %s", config.ExportPath)
			}
			if err := ExportToParquet(config.DB, config.ExportPath); err != nil {
				return fmt.Errorf("failed to export to Parquet: %w", err)
			}
			log.Printf("Successfully exported to %s", config.ExportPath)
		}
	}

	duration := time.Since(startTime)
	log.Printf("Import job completed in %s", duration.Round(time.Second))

	return nil
}
