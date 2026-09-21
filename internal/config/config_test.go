package config

import (
	"strings"
	"testing"
)

// setEnv sets every key in vars via t.Setenv, using "" to represent an
// unset variable (os.Getenv treats unset and empty-string identically,
// which is all Load cares about).
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func baseCognitoEnv() map[string]string {
	return map[string]string{
		"PORT":          "",
		"AUTH_PROVIDER": "",
		"AWS_REGION":    "ap-northeast-1",
		"DATABASE_URL":  "postgres://localhost/g5",
	}
}

func TestLoad_CognitoProviderSuccess(t *testing.T) {
	setEnv(t, baseCognitoEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
	if cfg.AuthProvider != AuthProviderCognito {
		t.Errorf("AuthProvider = %q, want %q", cfg.AuthProvider, AuthProviderCognito)
	}
	if cfg.AWSRegion != "ap-northeast-1" {
		t.Errorf("AWSRegion = %q, want %q", cfg.AWSRegion, "ap-northeast-1")
	}
	if cfg.DatabaseURL != "postgres://localhost/g5" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://localhost/g5")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	env := baseCognitoEnv()
	env["PORT"] = "9090"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
}

func TestLoad_MemoryProviderSuccess(t *testing.T) {
	setEnv(t, map[string]string{
		"PORT":          "",
		"AUTH_PROVIDER": "memory",
		"AWS_REGION":    "",
		"DATABASE_URL":  "postgres://localhost/g5",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.AuthProvider != AuthProviderMemory {
		t.Errorf("AuthProvider = %q, want %q", cfg.AuthProvider, AuthProviderMemory)
	}
}

func TestLoad_MissingRequiredVars(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name: "missing AWS_REGION for cognito provider",
			env: func() map[string]string {
				e := baseCognitoEnv()
				e["AWS_REGION"] = ""
				return e
			}(),
			wantErr: "AWS_REGION",
		},
		{
			name: "missing DATABASE_URL",
			env: func() map[string]string {
				e := baseCognitoEnv()
				e["DATABASE_URL"] = ""
				return e
			}(),
			wantErr: "DATABASE_URL",
		},
		{
			name: "missing DATABASE_URL for memory provider",
			env: map[string]string{
				"AUTH_PROVIDER": "memory",
				"DATABASE_URL":  "",
			},
			wantErr: "DATABASE_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() returned nil error, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoad_InvalidAuthProvider(t *testing.T) {
	env := baseCognitoEnv()
	env["AUTH_PROVIDER"] = "not-a-real-provider"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil error, want an error")
	}
	if !strings.Contains(err.Error(), "AUTH_PROVIDER") {
		t.Errorf("Load() error = %q, want it to mention AUTH_PROVIDER", err.Error())
	}
}
