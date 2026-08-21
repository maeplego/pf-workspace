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
	if cfg.Env != EnvDevelopment {
		t.Fatalf("env = %q", cfg.Env)
	}
}

func TestFromEnvStagingRejectsDevAuth(t *testing.T) {
	t.Setenv("WORKSPACE_ENV", "staging")
	t.Setenv("WORKSPACE_DEV_AUTH", "true")
	t.Setenv("OIDC_ISSUER", "http://idp.example")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error when staging enables WORKSPACE_DEV_AUTH")
	}
}

func TestFromEnvProductionRequiresOIDC(t *testing.T) {
	t.Setenv("WORKSPACE_ENV", "production")
	t.Setenv("WORKSPACE_DEV_AUTH", "false")
	_ = os.Unsetenv("OIDC_ISSUER")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error when production lacks OIDC_ISSUER")
	}
}

func TestFromEnvStagingOIDC(t *testing.T) {
	t.Setenv("WORKSPACE_ENV", "staging")
	t.Setenv("WORKSPACE_DEV_AUTH", "false")
	t.Setenv("OIDC_ISSUER", "http://idp.example")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != EnvStaging || cfg.DevAuth {
		t.Fatalf("%+v", cfg)
	}
}
