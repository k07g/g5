// Package integration exercises the whole server stack together — router,
// auth middleware, and the MongoDB-backed repository — wired up exactly as
// cmd/server/main.go wires them, then driven over real HTTP against a real
// MongoDB testcontainer. internal/api's own tests stub storage with an
// in-memory fake, and internal/db's tests exercise the repository alone;
// this package is the seam between the two, verifying they work together
// the way a caller of the running binary actually would.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"

	"github.com/k07g/g5/internal/api"
	"github.com/k07g/g5/internal/auth"
	"github.com/k07g/g5/internal/db"
	"github.com/k07g/g5/internal/models"
)

var testServer *httptest.Server

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests starts a MongoDB container, wires up the production router
// against it (using auth.MemoryVerifier in place of Cognito, since a real
// user pool isn't available in CI — the same substitution
// AUTH_PROVIDER=memory makes for local development), serves it over a real
// httptest.Server, and tears everything down afterward. It's its own
// function, not inlined into TestMain, so the deferred cleanup actually
// runs before the process exits.
func runTests(m *testing.M) int {
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:8")
	if err != nil {
		// No Docker available — skip rather than fail, so `go test ./...`
		// degrades gracefully instead of breaking everywhere.
		fmt.Fprintf(os.Stderr, "integration: skipping, failed to start MongoDB testcontainer (is Docker running?): %v\n", err)
		return 0
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "integration: failed to terminate MongoDB testcontainer: %v\n", err)
		}
	}()

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: failed to read MongoDB testcontainer connection string: %v\n", err)
		return 1
	}

	client, database, err := db.Connect(ctx, uri, "career_sheets_integration")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: failed to connect to MongoDB testcontainer: %v\n", err)
		return 1
	}
	defer client.Disconnect(ctx)

	careerSheets := db.NewCareerSheetRepository(database)
	verifier := auth.NewMemoryVerifier()
	handler := api.NewHandler(careerSheets)
	router := api.NewRouter(handler, api.AuthMiddleware(verifier))

	testServer = httptest.NewServer(router)
	defer testServer.Close()

	return m.Run()
}

func doRequest(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()

	var reqBody *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, testServer.URL+path, reqBody)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := testServer.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func sampleSheet() models.CareerSheet {
	sheet := models.CareerSheet{
		BasicInfo: models.BasicInfo{Name: "山田太郎", Email: "yamada@example.com"},
		Summary:   "summary",
	}
	sheet.Normalize()
	return sheet
}

func TestHealthz(t *testing.T) {
	resp := doRequest(t, http.MethodGet, "/healthz", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestCareerSheetLifecycle drives a single career sheet through its full
// lifecycle over real HTTP, against the real Mongo-backed repository —
// the same sequence verified manually with curl during development, now
// automated: create, read back, update in place, per-user scoping, delete,
// and idempotent re-delete.
func TestCareerSheetLifecycle(t *testing.T) {
	token := "integration-token-" + t.Name()

	t.Run("get without auth is rejected", func(t *testing.T) {
		resp := doRequest(t, http.MethodGet, "/career-sheet", "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("get before save is not found", func(t *testing.T) {
		resp := doRequest(t, http.MethodGet, "/career-sheet", token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	sheet := sampleSheet()

	t.Run("put creates the sheet", func(t *testing.T) {
		resp := doRequest(t, http.MethodPut, "/career-sheet", token, sheet)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}
	})

	t.Run("get returns the saved sheet", func(t *testing.T) {
		resp := doRequest(t, http.MethodGet, "/career-sheet", token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var got models.CareerSheet
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.BasicInfo.Name != sheet.BasicInfo.Name {
			t.Errorf("BasicInfo.Name = %q, want %q", got.BasicInfo.Name, sheet.BasicInfo.Name)
		}
	})

	t.Run("put again replaces the sheet in place", func(t *testing.T) {
		updated := sampleSheet()
		updated.BasicInfo.Name = "山田次郎"
		updated.Summary = "updated"

		resp := doRequest(t, http.MethodPut, "/career-sheet", token, updated)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}

		resp = doRequest(t, http.MethodGet, "/career-sheet", token, nil)
		var got models.CareerSheet
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.BasicInfo.Name != "山田次郎" || got.Summary != "updated" {
			t.Errorf("got = %+v, want the updated sheet's contents", got)
		}
	})

	t.Run("a different token does not see this user's sheet", func(t *testing.T) {
		resp := doRequest(t, http.MethodGet, "/career-sheet", "other-"+token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("delete removes the sheet", func(t *testing.T) {
		resp := doRequest(t, http.MethodDelete, "/career-sheet", token, nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}

		resp = doRequest(t, http.MethodGet, "/career-sheet", token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d (sheet should be gone)", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("deleting again is idempotent", func(t *testing.T) {
		resp := doRequest(t, http.MethodDelete, "/career-sheet", token, nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}
	})
}
