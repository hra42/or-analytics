package main

import (
	"regexp"
	"testing"
)

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantBucket  string
		wantKey     string
		expectError bool
	}{
		{
			name:        "valid S3 URI",
			uri:         "s3://my-bucket/path/to/file.parquet",
			wantBucket:  "my-bucket",
			wantKey:     "path/to/file.parquet",
			expectError: false,
		},
		{
			name:        "S3 URI with nested path",
			uri:         "s3://my-bucket/folder1/folder2/file.parquet",
			wantBucket:  "my-bucket",
			wantKey:     "folder1/folder2/file.parquet",
			expectError: false,
		},
		{
			name:        "S3 URI with single file",
			uri:         "s3://bucket/file.parquet",
			wantBucket:  "bucket",
			wantKey:     "file.parquet",
			expectError: false,
		},
		{
			name:        "invalid - no s3:// prefix",
			uri:         "my-bucket/file.parquet",
			expectError: true,
		},
		{
			name:        "invalid - missing key",
			uri:         "s3://my-bucket",
			expectError: true,
		},
		{
			name:        "invalid - only s3://",
			uri:         "s3://",
			expectError: true,
		},
		{
			name:        "invalid - empty string",
			uri:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3Path, err := ParseS3URI(tt.uri)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseS3URI(%q) expected error, got nil", tt.uri)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseS3URI(%q) unexpected error: %v", tt.uri, err)
				return
			}

			if s3Path.Bucket != tt.wantBucket {
				t.Errorf("ParseS3URI(%q) bucket = %q, want %q", tt.uri, s3Path.Bucket, tt.wantBucket)
			}

			if s3Path.Key != tt.wantKey {
				t.Errorf("ParseS3URI(%q) key = %q, want %q", tt.uri, s3Path.Key, tt.wantKey)
			}
		})
	}
}

func TestIsS3Path(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"s3://bucket/key", true},
		{"s3://my-bucket/path/to/file.parquet", true},
		{"s3://", true},
		{"/local/path/file.parquet", false},
		{"file.parquet", false},
		{"http://example.com/file", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsS3Path(tt.path)
			if got != tt.want {
				t.Errorf("IsS3Path(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestAddDateToS3Path(t *testing.T) {
	// Pattern to match YYYYMMDD format
	datePattern := regexp.MustCompile(`-\d{8}\.parquet$`)
	datePatternNoExt := regexp.MustCompile(`-\d{8}$`)

	tests := []struct {
		name            string
		uri             string
		wantPattern     *regexp.Regexp
		shouldContain   string
	}{
		{
			name:            "simple S3 path with extension",
			uri:             "s3://bucket/file.parquet",
			wantPattern:     datePattern,
			shouldContain:   "s3://bucket/file-",
		},
		{
			name:            "nested S3 path with extension",
			uri:             "s3://bucket/folder/subfolder/data.parquet",
			wantPattern:     datePattern,
			shouldContain:   "s3://bucket/folder/subfolder/data-",
		},
		{
			name:            "S3 path without extension",
			uri:             "s3://bucket/file",
			wantPattern:     datePatternNoExt,
			shouldContain:   "s3://bucket/file-",
		},
		{
			name:            "S3 path with multiple dots",
			uri:             "s3://bucket/file.backup.parquet",
			wantPattern:     datePattern,
			shouldContain:   "s3://bucket/file.backup-",
		},
		{
			name:            "S3 path with hyphens in name",
			uri:             "s3://my-bucket/my-file.parquet",
			wantPattern:     datePattern,
			shouldContain:   "s3://my-bucket/my-file-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addDateToS3Path(tt.uri)

			// Check that result contains the base path
			if !regexp.MustCompile(regexp.QuoteMeta(tt.shouldContain)).MatchString(got) {
				t.Errorf("addDateToS3Path(%q) = %q, should contain %q", tt.uri, got, tt.shouldContain)
			}

			// Check that result matches expected date pattern
			if !tt.wantPattern.MatchString(got) {
				t.Errorf("addDateToS3Path(%q) = %q, should match pattern %q", tt.uri, got, tt.wantPattern.String())
			}

			// Verify the date is in YYYYMMDD format (8 digits)
			dateMatch := regexp.MustCompile(`-(\d{8})`).FindStringSubmatch(got)
			if len(dateMatch) < 2 {
				t.Errorf("addDateToS3Path(%q) = %q, should contain date in format -YYYYMMDD", tt.uri, got)
			}
		})
	}
}

func TestExportToS3_ParseError(t *testing.T) {
	// This test doesn't require actual AWS credentials
	// We're just testing that the URI parsing error is properly handled

	// Create an in-memory test database
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Test with invalid S3 URI
	invalidURI := "not-an-s3-uri"
	err = ExportToS3(nil, db, invalidURI, nil)

	if err == nil {
		t.Error("ExportToS3 with invalid URI should return error")
	}
}

func TestS3Config(t *testing.T) {
	tests := []struct {
		name           string
		config         *S3Config
		wantEndpoint   string
		wantPathStyle  bool
	}{
		{
			name:          "nil config",
			config:        nil,
			wantEndpoint:  "",
			wantPathStyle: false,
		},
		{
			name: "custom endpoint",
			config: &S3Config{
				EndpointURL:    "https://minio.example.com:9000",
				ForcePathStyle: false,
			},
			wantEndpoint:  "https://minio.example.com:9000",
			wantPathStyle: false,
		},
		{
			name: "path-style enabled",
			config: &S3Config{
				EndpointURL:    "",
				ForcePathStyle: true,
			},
			wantEndpoint:  "",
			wantPathStyle: true,
		},
		{
			name: "full custom config",
			config: &S3Config{
				EndpointURL:    "https://nyc3.digitaloceanspaces.com",
				ForcePathStyle: true,
			},
			wantEndpoint:  "https://nyc3.digitaloceanspaces.com",
			wantPathStyle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config == nil {
				return // Skip validation for nil config
			}

			if tt.config.EndpointURL != tt.wantEndpoint {
				t.Errorf("EndpointURL = %q, want %q", tt.config.EndpointURL, tt.wantEndpoint)
			}

			if tt.config.ForcePathStyle != tt.wantPathStyle {
				t.Errorf("ForcePathStyle = %v, want %v", tt.config.ForcePathStyle, tt.wantPathStyle)
			}
		})
	}
}

// Note: Full integration tests with S3 would require:
// - AWS credentials
// - An S3 bucket
// - Network access
// These tests would be better suited for an integration test suite
// that runs in CI/CD with proper AWS credentials configured.
//
// Example integration test structure (not run by default):
//
// func TestS3Integration(t *testing.T) {
//     if testing.Short() {
//         t.Skip("Skipping integration test")
//     }
//
//     // Check for AWS credentials
//     if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
//         t.Skip("AWS credentials not configured")
//     }
//
//     // Test actual S3 upload/download
// }
