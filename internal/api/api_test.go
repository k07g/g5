package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/k07g/g5/internal/auth"
	"github.com/k07g/g5/internal/db"
	"github.com/k07g/g5/internal/models"
)

const testToken = "test-access-token"

// fakeCareerSheetStore is an in-memory CareerSheetStore for handler-level
// tests, so they don't need a live MongoDB deployment — the same role
// auth.MemoryVerifier plays for authentication in these tests.
type fakeCareerSheetStore struct {
	mu     sync.Mutex
	sheets map[string]models.CareerSheet
}

func newFakeCareerSheetStore() *fakeCareerSheetStore {
	return &fakeCareerSheetStore{sheets: make(map[string]models.CareerSheet)}
}

func (f *fakeCareerSheetStore) Get(_ context.Context, cognitoSub string) (*models.CareerSheet, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sheet, ok := f.sheets[cognitoSub]
	if !ok {
		return nil, db.ErrCareerSheetNotFound
	}
	return &sheet, nil
}

func (f *fakeCareerSheetStore) Upsert(_ context.Context, cognitoSub string, sheet *models.CareerSheet) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sheets[cognitoSub] = *sheet
	return nil
}

func (f *fakeCareerSheetStore) Delete(_ context.Context, cognitoSub string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.sheets[cognitoSub]; !ok {
		return db.ErrCareerSheetNotFound
	}
	delete(f.sheets, cognitoSub)
	return nil
}

func newTestServer(t *testing.T) (http.Handler, *fakeCareerSheetStore) {
	t.Helper()

	store := newFakeCareerSheetStore()
	verifier := auth.NewMemoryVerifier()
	handler := NewHandler(store)
	router := NewRouter(handler, AuthMiddleware(verifier))

	return router, store
}

func doRequest(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
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
	router, _ := newTestServer(t)

	rec := doRequest(t, router, http.MethodGet, "/healthz", nil, "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetCareerSheet(t *testing.T) {
	t.Run("missing authorization header is rejected", func(t *testing.T) {
		router, _ := newTestServer(t)
		rec := doRequest(t, router, http.MethodGet, "/career-sheet", nil, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("not found for a user with no saved sheet", func(t *testing.T) {
		router, _ := newTestServer(t)
		rec := doRequest(t, router, http.MethodGet, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("returns the saved sheet", func(t *testing.T) {
		router, store := newTestServer(t)
		sheet := sampleSheet()
		if err := store.Upsert(context.Background(), testToken, &sheet); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}

		rec := doRequest(t, router, http.MethodGet, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}

		var got models.CareerSheet
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.BasicInfo.Name != sheet.BasicInfo.Name {
			t.Errorf("BasicInfo.Name = %q, want %q", got.BasicInfo.Name, sheet.BasicInfo.Name)
		}
	})

	t.Run("sheets are scoped per user", func(t *testing.T) {
		router, store := newTestServer(t)
		sheet := sampleSheet()
		if err := store.Upsert(context.Background(), "other-user-token", &sheet); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}

		rec := doRequest(t, router, http.MethodGet, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestPutCareerSheet(t *testing.T) {
	t.Run("missing authorization header is rejected", func(t *testing.T) {
		router, _ := newTestServer(t)
		rec := doRequest(t, router, http.MethodPut, "/career-sheet", sampleSheet(), "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid body is rejected", func(t *testing.T) {
		router, _ := newTestServer(t)
		req := httptest.NewRequest(http.MethodPut, "/career-sheet", bytes.NewBufferString("not json"))
		req.Header.Set("Authorization", "Bearer "+testToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("valid sheet is saved and can be read back", func(t *testing.T) {
		router, _ := newTestServer(t)

		rec := doRequest(t, router, http.MethodPut, "/career-sheet", sampleSheet(), testToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}

		rec = doRequest(t, router, http.MethodGet, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}

func TestDeleteCareerSheet(t *testing.T) {
	t.Run("missing authorization header is rejected", func(t *testing.T) {
		router, _ := newTestServer(t)
		rec := doRequest(t, router, http.MethodDelete, "/career-sheet", nil, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("deleting an existing sheet succeeds", func(t *testing.T) {
		router, store := newTestServer(t)
		sheet := sampleSheet()
		if err := store.Upsert(context.Background(), testToken, &sheet); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}

		rec := doRequest(t, router, http.MethodDelete, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}

		rec = doRequest(t, router, http.MethodGet, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d (sheet should be gone)", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("deleting an already-absent sheet is idempotent", func(t *testing.T) {
		router, _ := newTestServer(t)

		rec := doRequest(t, router, http.MethodDelete, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}
