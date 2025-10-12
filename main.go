package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hra42/openrouter-go"
	_ "github.com/marcboeker/go-duckdb"
)

const defaultDBPath = "analytics.db"

func main() {
	// Command-line flags
	dateFilter := flag.String("date", "", "Filter by specific date (YYYY-MM-DD format)")
	dbPathFlag := flag.String("db", defaultDBPath, "Path to DuckDB database file")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	exportPath := flag.String("export", "", "Export data to Parquet file at specified path (local or s3://)")
	s3Endpoint := flag.String("s3-endpoint", "", "Custom S3 endpoint URL for S3-compatible services (e.g., MinIO, DigitalOcean Spaces)")
	s3ForcePathStyle := flag.Bool("s3-path-style", false, "Use path-style S3 URLs (required for MinIO and some S3-compatible services)")

	// Scheduler flags
	schedule := flag.String("schedule", "", "Run as a scheduler with specified schedule (daily, hourly, now, or cron expression)")
	timezone := flag.String("timezone", "UTC", "Timezone for scheduler (e.g., America/New_York, Europe/London)")
	webhookURL := flag.String("webhook-url", "", "Webhook URL to send notifications after each scheduled run")

	flag.Parse()

	// Check if running in scheduler mode
	if *schedule != "" {
		runSchedulerMode(*schedule, *timezone, *dateFilter, *dbPathFlag, *exportPath, *s3Endpoint, *s3ForcePathStyle, *verbose, *webhookURL)
		return
	}

	// Run in one-time mode (original behavior)
	runOnceMode(*dateFilter, *dbPathFlag, *exportPath, *s3Endpoint, *s3ForcePathStyle, *verbose)
}

// runSchedulerMode runs the application as a scheduler
func runSchedulerMode(scheduleExpr, timezone, dateFilter, dbPath, exportPath, s3Endpoint string, s3PathStyle, verbose bool, webhookURL string) {
	ctx := context.Background()

	// Create S3 config if needed
	var s3Config *S3Config
	if exportPath != "" && IsS3Path(exportPath) {
		s3Config = &S3Config{
			EndpointURL:    s3Endpoint,
			ForcePathStyle: s3PathStyle,
		}
	}

	// Create scheduler config
	config := &SchedulerConfig{
		Schedule:   scheduleExpr,
		DateFilter: dateFilter,
		DBPath:     dbPath,
		ExportPath: exportPath,
		S3Config:   s3Config,
		Verbose:    verbose,
		Timezone:   timezone,
		WebhookURL: webhookURL,
	}

	// Run scheduler
	if err := RunScheduler(ctx, config); err != nil {
		log.Fatalf("Scheduler error: %v", err)
	}
}

// runOnceMode runs the application once (original behavior)
func runOnceMode(dateFilter, dbPath, exportPath, s3Endpoint string, s3PathStyle, verbose bool) {
	// Get API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	fmt.Println("OpenRouter Analytics - Activity Data Importer")
	fmt.Println("==============================================")
	fmt.Println()

	// Create OpenRouter client
	client := openrouter.NewClient(
		openrouter.WithAPIKey(apiKey),
		openrouter.WithReferer("https://github.com/hra42/or-analytics"),
		openrouter.WithAppName("OR Analytics"),
	)

	ctx := context.Background()

	// Connect to DuckDB and initialize table
	db, err := InitDB(dbPath)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	if verbose {
		fmt.Println("✓ Database connection established")
		fmt.Println("✓ Activity table ready")
		fmt.Println()
	}

	// Fetch activity data
	var options *openrouter.ActivityOptions
	if dateFilter != "" {
		options = &openrouter.ActivityOptions{
			Date: dateFilter,
		}
		fmt.Printf("Fetching activity data for: %s\n", dateFilter)
	} else {
		fmt.Println("Fetching all activity data (last 30 completed UTC days)")
	}

	activity, err := client.GetActivity(ctx, options)
	if err != nil {
		// Check if it's a provisioning key error
		if reqErr, ok := err.(*openrouter.RequestError); ok {
			if reqErr.StatusCode == 401 || reqErr.StatusCode == 403 {
				fmt.Println("❌ Error: This endpoint requires a provisioning key.")
				fmt.Println("Please create one at: https://openrouter.ai/settings/provisioning-keys")
				fmt.Println("Then set it as your OPENROUTER_API_KEY environment variable.")
				os.Exit(1)
			}
		}
		log.Fatalf("Error getting activity: %v", err)
	}

	fmt.Printf("Retrieved %d activity records\n", len(activity.Data))

	if len(activity.Data) == 0 {
		fmt.Println("No activity data found. This is normal for new accounts.")
		return
	}

	// Convert and insert records
	records := ConvertActivityData(activity.Data)
	inserted := 0

	// Insert in batches with progress reporting
	for i := 0; i < len(records); i += 100 {
		end := i + 100
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		count, err := InsertActivityRecords(db, batch)
		inserted += count

		if err != nil {
			log.Printf("Warning: error inserting batch: %v", err)
		}

		if verbose && inserted%100 == 0 {
			fmt.Printf("  Processed %d records...\n", inserted)
		}
	}

	fmt.Printf("✓ Successfully imported %d records\n\n", inserted)

	// Export to Parquet if requested
	if exportPath != "" {
		// Check if it's an S3 path
		if IsS3Path(exportPath) {
			// Create S3 config
			s3Config := &S3Config{
				EndpointURL:    s3Endpoint,
				ForcePathStyle: s3PathStyle,
			}

			// Display endpoint info if custom endpoint is used
			if s3Endpoint != "" {
				fmt.Printf("Using custom S3 endpoint: %s\n", s3Endpoint)
			}
			if s3PathStyle {
				fmt.Printf("Using path-style S3 URLs\n")
			}

			fmt.Printf("Exporting data to S3: %s\n", exportPath)
			if err := ExportToS3(ctx, db, exportPath, s3Config); err != nil {
				log.Fatalf("Error exporting to S3: %v", err)
			}
			fmt.Printf("✓ Successfully exported to %s\n\n", exportPath)
		} else {
			fmt.Printf("Exporting data to Parquet file: %s\n", exportPath)
			if err := ExportToParquet(db, exportPath); err != nil {
				log.Fatalf("Error exporting to Parquet: %v", err)
			}
			fmt.Printf("✓ Successfully exported to %s\n\n", exportPath)
		}
	}

	// Display summary statistics
	summary, err := GetSummary(db)
	if err != nil {
		log.Printf("Error getting summary: %v", err)
		return
	}

	PrintSummary(summary, dbPath)
}

// PrintSummary displays summary statistics to the console
func PrintSummary(summary *Summary, dbPath string) {
	fmt.Println("Database Summary")
	fmt.Println("================")
	fmt.Printf("Total records in database: %d\n", summary.TotalRecords)
	fmt.Printf("Date range: %d unique dates\n", summary.UniqueDates)
	fmt.Printf("Models used: %d unique models\n", summary.UniqueModels)
	fmt.Printf("Providers: %d unique providers\n\n", summary.UniqueProviders)

	fmt.Printf("Total API requests: %.0f\n", summary.TotalRequests)
	fmt.Printf("Total usage cost: $%.4f\n", summary.TotalUsage)
	fmt.Printf("Total tokens:\n")
	fmt.Printf("  Prompt: %.0f\n", summary.TotalPromptTokens)
	fmt.Printf("  Completion: %.0f\n", summary.TotalCompletionTokens)
	if summary.TotalReasoningTokens > 0 {
		fmt.Printf("  Reasoning: %.0f\n", summary.TotalReasoningTokens)
	}

	fmt.Println()
	fmt.Printf("Database saved to: %s\n", dbPath)
	fmt.Println()
	fmt.Println("You can now query the database using DuckDB CLI or SQL queries.")
	fmt.Println("See the queries/ directory for example queries.")
}
