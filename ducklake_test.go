package main

import (
	"testing"
)

func TestBuildPostgresConnStr(t *testing.T) {
	tests := []struct {
		name     string
		dbname   string
		host     string
		port     string
		user     string
		password string
		want     string
	}{
		{
			name:     "all parameters",
			dbname:   "testdb",
			host:     "localhost",
			port:     "5432",
			user:     "admin",
			password: "secret",
			want:     "dbname=testdb host=localhost port=5432 user=admin password=secret",
		},
		{
			name:     "minimal parameters",
			dbname:   "testdb",
			host:     "localhost",
			port:     "",
			user:     "",
			password: "",
			want:     "dbname=testdb host=localhost",
		},
		{
			name:     "empty string",
			dbname:   "",
			host:     "",
			port:     "",
			user:     "",
			password: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPostgresConnStr(tt.dbname, tt.host, tt.port, tt.user, tt.password)
			if got != tt.want {
				t.Errorf("BuildPostgresConnStr() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Note: Full integration tests for DuckLake would require:
// - Running PostgreSQL instance for catalog
// - S3/R2 credentials and bucket access
// - DuckLake extensions installed in DuckDB
//
// These are better suited for CI/CD or manual testing environments
// with proper credentials configured.
//
// For unit testing, we test the helper functions and ensure
// the configuration structure is correct.

func TestDuckLakeConfig(t *testing.T) {
	config := &DuckLakeConfig{
		Enabled:         true,
		PostgresConnStr: "dbname=test host=localhost",
		DatabaseName:    "test_db",
		S3AccessKey:     "test_key",
		S3SecretKey:     "test_secret",
		S3Endpoint:      "s3.example.com",
		S3Bucket:        "test-bucket",
		S3Region:        "us-east-1",
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if config.DatabaseName != "test_db" {
		t.Errorf("Expected DatabaseName to be 'test_db', got %q", config.DatabaseName)
	}
	if config.S3Bucket != "test-bucket" {
		t.Errorf("Expected S3Bucket to be 'test-bucket', got %q", config.S3Bucket)
	}
}
