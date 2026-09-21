package config

import (
	"fmt"
	"os"
)

// AuthProvider selects the auth.Verifier implementation used by the server.
type AuthProvider string

const (
	// AuthProviderCognito verifies access tokens against a real Amazon
	// Cognito user pool (the same one github.com/k07g/g4 issues tokens
	// from).
	AuthProviderCognito AuthProvider = "cognito"
	// AuthProviderMemory is a stand-in for local development and testing
	// that requires no AWS account.
	AuthProviderMemory AuthProvider = "memory"
)

type Config struct {
	Port          string
	AuthProvider  AuthProvider
	AWSRegion     string
	MongoURI      string
	MongoDatabase string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		AuthProvider:  AuthProvider(getEnv("AUTH_PROVIDER", string(AuthProviderCognito))),
		AWSRegion:     os.Getenv("AWS_REGION"),
		MongoURI:      os.Getenv("MONGODB_URI"),
		MongoDatabase: os.Getenv("MONGODB_DATABASE"),
	}

	var missing []string
	if cfg.MongoURI == "" {
		missing = append(missing, "MONGODB_URI")
	}
	if cfg.MongoDatabase == "" {
		missing = append(missing, "MONGODB_DATABASE")
	}

	switch cfg.AuthProvider {
	case AuthProviderCognito:
		if cfg.AWSRegion == "" {
			missing = append(missing, "AWS_REGION")
		}
	case AuthProviderMemory:
		// No external credentials required.
	default:
		return nil, fmt.Errorf("invalid AUTH_PROVIDER %q: must be %q or %q", cfg.AuthProvider, AuthProviderCognito, AuthProviderMemory)
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
