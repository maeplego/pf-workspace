package config

import (
	"os"
	"testing"
)

func TestFromEnvDatabaseURL(t *testing.T) {
	t.Setenv("WORKSPACE_DEV_AUTH", "true")
	t.Setenv("WORKSPACE_DATABASE_URL", " postgres://workspace:workspace@localhost:5439/workspace ")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://workspace:workspace@localhost:5439/workspace" {
		t.Fatalf("database url: %q", cfg.DatabaseURL)
	}
}

func TestFromEnvEmptyDatabaseUsesMemory(t *testing.T) {
	t.Setenv("WORKSPACE_DEV_AUTH", "true")
	_ = os.Unsetenv("WORKSPACE_DATABASE_URL")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty database url, got %q", cfg.DatabaseURL)
	}
}
