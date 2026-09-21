package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/k07g/g5/internal/auth"
	"github.com/k07g/g5/internal/db"
	"github.com/k07g/g5/internal/models"
)

const testToken = "test-access-token"

func newTestServer(t *testing.T) (http.Handler, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { mockDB.Close() })

	verifier := auth.NewMemoryVerifier()
	careerSheets := db.NewCareerSheetRepository(mockDB)
	handler := NewHandler(careerSheets)
	router := NewRouter(handler, AuthMiddleware(verifier))

	return router, mock
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
		router, mock := newTestServer(t)
		mock.ExpectQuery("SELECT data FROM career_sheets").
			WithArgs(testToken).
			WillReturnError(sql.ErrNoRows)

		rec := doRequest(t, router, http.MethodGet, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("returns the saved sheet", func(t *testing.T) {
		router, mock := newTestServer(t)
		sheet := sampleSheet()
		raw, err := json.Marshal(sheet)
		if err != nil {
			t.Fatalf("failed to marshal sheet: %v", err)
		}
		rows := sqlmock.NewRows([]string{"data"}).AddRow(raw)
		mock.ExpectQuery("SELECT data FROM career_sheets").
			WithArgs(testToken).
			WillReturnRows(rows)

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

	t.Run("valid sheet is saved", func(t *testing.T) {
		router, mock := newTestServer(t)
		mock.ExpectExec("INSERT INTO career_sheets").
			WillReturnResult(sqlmock.NewResult(0, 1))

		rec := doRequest(t, router, http.MethodPut, "/career-sheet", sampleSheet(), testToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
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
		router, mock := newTestServer(t)
		mock.ExpectExec("DELETE FROM career_sheets").
			WithArgs(testToken).
			WillReturnResult(sqlmock.NewResult(0, 1))

		rec := doRequest(t, router, http.MethodDelete, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("deleting an already-absent sheet is idempotent", func(t *testing.T) {
		router, mock := newTestServer(t)
		mock.ExpectExec("DELETE FROM career_sheets").
			WithArgs(testToken).
			WillReturnResult(sqlmock.NewResult(0, 0))

		rec := doRequest(t, router, http.MethodDelete, "/career-sheet", nil, testToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}
