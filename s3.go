package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Path represents a parsed S3 URI
type S3Path struct {
	Bucket string
	Key    string
}

// ParseS3URI parses an S3 URI in the format s3://bucket/key
func ParseS3URI(uri string) (*S3Path, error) {
	if !strings.HasPrefix(uri, "s3://") {
		return nil, fmt.Errorf("invalid S3 URI: must start with s3://")
	}

	// Remove s3:// prefix
	path := strings.TrimPrefix(uri, "s3://")

	// Split into bucket and key
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid S3 URI: must include bucket and key (s3://bucket/key)")
	}

	return &S3Path{
		Bucket: parts[0],
		Key:    parts[1],
	}, nil
}

// IsS3Path checks if a path is an S3 URI
func IsS3Path(path string) bool {
	return strings.HasPrefix(path, "s3://")
}

// addDateToS3Path adds the current date to the S3 path before the file extension
// Example: s3://bucket/file.parquet -> s3://bucket/file-20250112.parquet
func addDateToS3Path(s3URI string) string {
	// Get current date in YYYYMMDD format
	currentDate := time.Now().UTC().Format("20060102")

	// Find the last slash to separate path from filename
	lastSlashIdx := strings.LastIndex(s3URI, "/")
	if lastSlashIdx == -1 {
		// No slash found, just append date (shouldn't happen with valid S3 URIs)
		return s3URI + "-" + currentDate
	}

	// Split into path and filename
	pathPart := s3URI[:lastSlashIdx+1]
	filename := s3URI[lastSlashIdx+1:]

	// Find the extension
	extIdx := strings.LastIndex(filename, ".")
	if extIdx == -1 {
		// No extension, append date at the end
		return pathPart + filename + "-" + currentDate
	}

	// Insert date before the extension
	nameWithoutExt := filename[:extIdx]
	ext := filename[extIdx:]
	return pathPart + nameWithoutExt + "-" + currentDate + ext
}

// S3Config holds configuration for S3 uploads
type S3Config struct {
	EndpointURL    string // Custom endpoint URL for S3-compatible services
	ForcePathStyle bool   // Use path-style addressing (required for MinIO, etc.)
}

// UploadToS3 uploads a file to S3 with optional custom configuration
func UploadToS3(ctx context.Context, filePath string, s3URI string, cfg *S3Config) error {
	// Parse S3 URI
	s3Path, err := ParseS3URI(s3URI)
	if err != nil {
		return err
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	clientOptions := []func(*s3.Options){}

	// Apply custom endpoint if provided
	if cfg != nil && cfg.EndpointURL != "" {
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		})
	}

	// Apply path-style addressing if requested
	if cfg != nil && cfg.ForcePathStyle {
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Create S3 client with options
	client := s3.NewFromConfig(awsCfg, clientOptions...)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create uploader
	uploader := manager.NewUploader(client)

	// Upload the file
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s3Path.Bucket),
		Key:           aws.String(s3Path.Key),
		Body:          file,
		ContentType:   aws.String("application/octet-stream"),
		ContentLength: aws.Int64(fileInfo.Size()),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// ExportToS3 exports data to a Parquet file and uploads it to S3
// The file is created locally in a temp directory and then uploaded
// The S3 URI is modified to include the current date before the file extension
func ExportToS3(ctx context.Context, db *sql.DB, s3URI string, cfg *S3Config) error {
	// Create temporary file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "or-analytics-export.parquet")

	// Ensure we clean up the temp file
	defer os.Remove(tempFile)

	// Export to local Parquet file first
	query := fmt.Sprintf("COPY activity TO '%s' (FORMAT PARQUET)", tempFile)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to export to parquet: %w", err)
	}

	// Add date to the S3 URI filename
	s3URIWithDate := addDateToS3Path(s3URI)

	// Upload to S3
	if err := UploadToS3(ctx, tempFile, s3URIWithDate, cfg); err != nil {
		return err
	}

	return nil
}

// DownloadFromS3 downloads a file from S3 (utility function for testing/future use)
func DownloadFromS3(ctx context.Context, s3URI string, destPath string, cfg *S3Config) error {
	// Parse S3 URI
	s3Path, err := ParseS3URI(s3URI)
	if err != nil {
		return err
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	clientOptions := []func(*s3.Options){}

	// Apply custom endpoint if provided
	if cfg != nil && cfg.EndpointURL != "" {
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		})
	}

	// Apply path-style addressing if requested
	if cfg != nil && cfg.ForcePathStyle {
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Create S3 client with options
	client := s3.NewFromConfig(awsCfg, clientOptions...)

	// Create downloader
	downloader := manager.NewDownloader(client)

	// Create the destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	// Download the file
	_, err = downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(s3Path.Bucket),
		Key:    aws.String(s3Path.Key),
	})
	if err != nil {
		return fmt.Errorf("failed to download from S3: %w", err)
	}

	return nil
}

// ReadFileContents reads and returns file contents (utility for testing)
func ReadFileContents(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}
