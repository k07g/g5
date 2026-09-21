package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/k07g/g5/internal/models"
)

func newMockCareerSheetRepo(t *testing.T) (*CareerSheetRepository, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { mockDB.Close() })

	return NewCareerSheetRepository(mockDB), mock
}

func sampleSheet() *models.CareerSheet {
	sheet := &models.CareerSheet{
		BasicInfo: models.BasicInfo{Name: "山田太郎", Email: "yamada@example.com"},
		Summary:   "summary",
	}
	sheet.Normalize()
	return sheet
}

func TestCareerSheetRepository_Get(t *testing.T) {
	repo, mock := newMockCareerSheetRepo(t)

	t.Run("found", func(t *testing.T) {
		raw, err := json.Marshal(sampleSheet())
		if err != nil {
			t.Fatalf("failed to marshal sample sheet: %v", err)
		}
		rows := sqlmock.NewRows([]string{"data"}).AddRow(raw)

		mock.ExpectQuery("SELECT data FROM career_sheets").
			WithArgs("sub-123").
			WillReturnRows(rows)

		sheet, err := repo.Get(context.Background(), "sub-123")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if sheet.BasicInfo.Name != "山田太郎" {
			t.Errorf("BasicInfo.Name = %q, want %q", sheet.BasicInfo.Name, "山田太郎")
		}
		if sheet.WorkExperiences == nil {
			t.Error("WorkExperiences should be normalized to a non-nil empty slice")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT data FROM career_sheets").
			WithArgs("missing-sub").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.Get(context.Background(), "missing-sub")
		if !errors.Is(err, ErrCareerSheetNotFound) {
			t.Errorf("Get error = %v, want %v", err, ErrCareerSheetNotFound)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCareerSheetRepository_Upsert(t *testing.T) {
	repo, mock := newMockCareerSheetRepo(t)
	sheet := sampleSheet()
	raw, err := json.Marshal(sheet)
	if err != nil {
		t.Fatalf("failed to marshal sample sheet: %v", err)
	}

	mock.ExpectExec("INSERT INTO career_sheets").
		WithArgs("sub-123", raw).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Upsert(context.Background(), "sub-123", sheet); err != nil {
		t.Errorf("Upsert returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCareerSheetRepository_Delete(t *testing.T) {
	repo, mock := newMockCareerSheetRepo(t)

	t.Run("deletes existing sheet", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM career_sheets").
			WithArgs("sub-123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := repo.Delete(context.Background(), "sub-123"); err != nil {
			t.Errorf("Delete returned error: %v", err)
		}
	})

	t.Run("returns ErrCareerSheetNotFound when nothing deleted", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM career_sheets").
			WithArgs("missing-sub").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Delete(context.Background(), "missing-sub")
		if !errors.Is(err, ErrCareerSheetNotFound) {
			t.Errorf("Delete error = %v, want %v", err, ErrCareerSheetNotFound)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
