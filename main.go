package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/hra42/openrouter-go"
)

const defaultDBPath = "analytics.db"

func main() {
	// Command-line flags
	dateFilter := flag.String("date", "", "Filter by specific date (YYYY-MM-DD format)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")

	// Scheduler flags
	schedule := flag.String("schedule", "", "Run as a scheduler with specified schedule (daily, hourly, now, or cron expression)")
	timezone := flag.String("timezone", "UTC", "Timezone for scheduler (e.g., America/New_York, Europe/London)")
	webhookURL := flag.String("webhook-url", "", "Webhook URL to send notifications after each scheduled run")

	// DuckLake configuration
	duckLakeDB := flag.String("db", "or_analytics", "DuckLake database name")
	pgHost := flag.String("pg-host", "localhost", "PostgreSQL catalog host")
	pgPort := flag.String("pg-port", "5432", "PostgreSQL catalog port")
	pgUser := flag.String("pg-user", "admin", "PostgreSQL catalog user")
	pgPassword := flag.String("pg-password", "", "PostgreSQL catalog password (or use PG_PASSWORD env var)")
	pgDBName := flag.String("pg-dbname", "or_analytics_catalog", "PostgreSQL catalog database name")
	s3Key := flag.String("s3-key", "", "S3/R2 access key ID (or use S3_KEY env var)")
	s3Secret := flag.String("s3-secret", "", "S3/R2 secret access key (or use S3_SECRET env var)")
	s3Endpoint := flag.String("s3-endpoint", "", "S3/R2 endpoint URL")
	s3Bucket := flag.String("s3-bucket", "or-analytics", "S3/R2 bucket name")
	s3Region := flag.String("s3-region", "us-east-1", "S3/R2 region")

	flag.Parse()

	// Get credentials from flags or environment
	password := *pgPassword
	if password == "" {
		password = os.Getenv("PG_PASSWORD")
	}

	accessKey := *s3Key
	if accessKey == "" {
		accessKey = os.Getenv("S3_KEY")
	}

	secretKey := *s3Secret
	if secretKey == "" {
		secretKey = os.Getenv("S3_SECRET")
	}

	// Validate required credentials
	if password == "" {
		log.Fatal("PostgreSQL password required: use -pg-password flag or PG_PASSWORD env var")
	}
	if accessKey == "" {
		log.Fatal("S3 access key required: use -s3-key flag or S3_KEY env var")
	}
	if secretKey == "" {
		log.Fatal("S3 secret key required: use -s3-secret flag or S3_SECRET env var")
	}

	// Build DuckLake configuration
	pgConnStr := BuildPostgresConnStr(*pgDBName, *pgHost, *pgPort, *pgUser, password)

	config := &DuckLakeConfig{
		Enabled:         true,
		PostgresConnStr: pgConnStr,
		DatabaseName:    *duckLakeDB,
		S3AccessKey:     accessKey,
		S3SecretKey:     secretKey,
		S3Endpoint:      *s3Endpoint,
		S3Bucket:        *s3Bucket,
		S3Region:        *s3Region,
	}

	// Check if running in scheduler mode
	if *schedule != "" {
		runSchedulerMode(*schedule, *timezone, *dateFilter, *verbose, *webhookURL, config)
		return
	}

	// Run in one-time mode
	runOnceMode(*dateFilter, *verbose, config)
}

// runSchedulerMode runs the application as a scheduler
func runSchedulerMode(scheduleExpr, timezone, dateFilter string, verbose bool, webhookURL string, duckLakeConfig *DuckLakeConfig) {
	ctx := context.Background()

	// Create scheduler config
	config := &SchedulerConfig{
		Schedule:       scheduleExpr,
		DateFilter:     dateFilter,
		Verbose:        verbose,
		Timezone:       timezone,
		WebhookURL:     webhookURL,
		DuckLakeConfig: duckLakeConfig,
	}

	// Run scheduler
	if err := RunScheduler(ctx, config); err != nil {
		log.Fatalf("Scheduler error: %v", err)
	}
}

// runOnceMode runs the application once
func runOnceMode(dateFilter string, verbose bool, config *DuckLakeConfig) {
	// Get API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	fmt.Println("OpenRouter Analytics - DuckLake Incremental Importer")
	fmt.Println("=====================================================")
	fmt.Println()

	// Create OpenRouter client
	client := openrouter.NewClient(
		openrouter.WithAPIKey(apiKey),
		openrouter.WithReferer("https://github.com/hra42/or-analytics"),
		openrouter.WithAppName("OR Analytics"),
	)

	ctx := context.Background()

	// Initialize DuckLake connection (in-memory, no local persistence)
	db, err := InitDuckLake(config)
	if err != nil {
		log.Fatalf("Error initializing DuckLake: %v", err)
	}
	defer db.Close()

	if verbose {
		fmt.Println("✓ DuckLake connection established (in-memory)")
		fmt.Printf("✓ Connected to remote catalog: %s\n", config.DatabaseName)
		fmt.Printf("✓ Data storage: s3://%s\n", config.S3Bucket)
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

	// Convert and append records incrementally
	records := ConvertActivityData(activity.Data)

	lastDate, err := GetLastDuckLakeDate(db, config.DatabaseName)
	if err != nil {
		log.Printf("Warning: could not get last date from DuckLake: %v", err)
	}

	if lastDate != "" && verbose {
		fmt.Printf("Last date in DuckLake: %s\n", lastDate)
		fmt.Println("Only importing records newer than this date...")
	}

	// Append incrementally (only new records)
	inserted, err := AppendToDuckLake(db, config.DatabaseName, records)
	if err != nil {
		log.Fatalf("Error appending to DuckLake: %v", err)
	}

	if inserted == 0 {
		fmt.Println("No new records to append (all data already exists in DuckLake)")
	} else {
		fmt.Printf("✓ Successfully appended %d new records to DuckLake\n\n", inserted)
	}

	// Display summary statistics
	summary, err := GetDuckLakeSummary(db, config.DatabaseName)
	if err != nil {
		log.Printf("Error getting DuckLake summary: %v", err)
		return
	}

	PrintSummary(summary, fmt.Sprintf("DuckLake: %s (s3://%s)", config.DatabaseName, config.S3Bucket))
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
