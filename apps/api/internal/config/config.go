package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	DevAuth          bool
	OIDCIssuer       string
	OIDCInternalBase string
	OIDCAudience     string
	CORSOrigin       string
	InternalToken    string
	MediaAPIURL      string
	PublicURL        string
	UploadDir        string
}

func FromEnv() (Config, error) {
	port := strings.TrimSpace(os.Getenv("WORKSPACE_HTTP_PORT"))
	if port == "" {
		port = "8096"
	}
	devAuth := strings.EqualFold(os.Getenv("WORKSPACE_DEV_AUTH"), "true") || os.Getenv("WORKSPACE_DEV_AUTH") == "1"
	cfg := Config{
		HTTPAddr:         ":" + port,
		DatabaseURL:      strings.TrimSpace(os.Getenv("WORKSPACE_DATABASE_URL")),
		DevAuth:          devAuth,
		OIDCIssuer:       strings.TrimSpace(os.Getenv("OIDC_ISSUER")),
		OIDCInternalBase: strings.TrimSpace(os.Getenv("OIDC_INTERNAL_BASE")),
		OIDCAudience:     strings.TrimSpace(os.Getenv("OIDC_AUDIENCE")),
		CORSOrigin:       strings.TrimSpace(os.Getenv("WORKSPACE_CORS_ORIGIN")),
		InternalToken:    strings.TrimSpace(os.Getenv("WORKSPACE_INTERNAL_TOKEN")),
		MediaAPIURL:      strings.TrimSpace(os.Getenv("MEDIA_API_URL")),
		PublicURL:        strings.TrimSpace(os.Getenv("WORKSPACE_PUBLIC_URL")),
		UploadDir:        strings.TrimSpace(os.Getenv("WORKSPACE_UPLOAD_DIR")),
	}
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "http://localhost:3006"
	}
	if cfg.OIDCIssuer == "" && !cfg.DevAuth {
		return cfg, fmt.Errorf("OIDC_ISSUER or WORKSPACE_DEV_AUTH=true is required")
	}
	return cfg, nil
}

func ParseBoolEnv(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
