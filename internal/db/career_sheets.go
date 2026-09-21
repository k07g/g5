package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/k07g/g5/internal/models"
)

var ErrCareerSheetNotFound = errors.New("career sheet not found")

type CareerSheetRepository struct {
	db *sql.DB
}

func NewCareerSheetRepository(db *sql.DB) *CareerSheetRepository {
	return &CareerSheetRepository{db: db}
}

// Get returns the career sheet belonging to cognitoSub, or
// ErrCareerSheetNotFound if none has been saved yet.
func (r *CareerSheetRepository) Get(ctx context.Context, cognitoSub string) (*models.CareerSheet, error) {
	const q = `SELECT data FROM career_sheets WHERE cognito_sub = $1`

	var raw []byte
	err := r.db.QueryRowContext(ctx, q, cognitoSub).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCareerSheetNotFound
	}
	if err != nil {
		return nil, err
	}

	var sheet models.CareerSheet
	if err := json.Unmarshal(raw, &sheet); err != nil {
		return nil, err
	}
	sheet.Normalize()
	return &sheet, nil
}

// Upsert replaces the entire career sheet for cognitoSub, creating it if it
// doesn't exist yet. The frontend always sends the full document (it has
// no partial-update UI), so a whole-document replace is all this needs.
func (r *CareerSheetRepository) Upsert(ctx context.Context, cognitoSub string, sheet *models.CareerSheet) error {
	raw, err := json.Marshal(sheet)
	if err != nil {
		return err
	}

	const q = `
		INSERT INTO career_sheets (cognito_sub, data)
		VALUES ($1, $2)
		ON CONFLICT (cognito_sub)
		DO UPDATE SET data = EXCLUDED.data, updated_at = now()
	`
	_, err = r.db.ExecContext(ctx, q, cognitoSub, raw)
	return err
}

// Delete removes the career sheet for cognitoSub, if any.
func (r *CareerSheetRepository) Delete(ctx context.Context, cognitoSub string) error {
	const q = `DELETE FROM career_sheets WHERE cognito_sub = $1`

	res, err := r.db.ExecContext(ctx, q, cognitoSub)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrCareerSheetNotFound
	}
	return nil
}
