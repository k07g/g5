package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/k07g/g5/internal/db"
	"github.com/k07g/g5/internal/models"
)

// CareerSheetStore is the storage dependency Handler needs. db.CareerSheetRepository
// (backed by MongoDB) is the production implementation; tests use an
// in-memory fake so handler behavior can be verified without a live
// database, the same way AuthMiddleware is tested against auth.MemoryVerifier.
type CareerSheetStore interface {
	Get(ctx context.Context, cognitoSub string) (*models.CareerSheet, error)
	Upsert(ctx context.Context, cognitoSub string, sheet *models.CareerSheet) error
	Delete(ctx context.Context, cognitoSub string) error
}

type Handler struct {
	careerSheets CareerSheetStore
}

func NewHandler(careerSheets CareerSheetStore) *Handler {
	return &Handler{careerSheets: careerSheets}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func cognitoSub(r *http.Request) string {
	sub, _ := r.Context().Value(contextKeyCognitoSub).(string)
	return sub
}

// --- Get career sheet ---

func (h *Handler) GetCareerSheet(w http.ResponseWriter, r *http.Request) {
	sheet, err := h.careerSheets.Get(r.Context(), cognitoSub(r))
	if errors.Is(err, db.ErrCareerSheetNotFound) {
		writeError(w, http.StatusNotFound, "career sheet not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load career sheet")
		return
	}
	writeJSON(w, http.StatusOK, sheet)
}

// --- Save (create or replace) career sheet ---

func (h *Handler) PutCareerSheet(w http.ResponseWriter, r *http.Request) {
	var sheet models.CareerSheet
	if err := json.NewDecoder(r.Body).Decode(&sheet); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sheet.Normalize()

	if err := h.careerSheets.Upsert(r.Context(), cognitoSub(r), &sheet); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save career sheet")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Delete career sheet ---

func (h *Handler) DeleteCareerSheet(w http.ResponseWriter, r *http.Request) {
	err := h.careerSheets.Delete(r.Context(), cognitoSub(r))
	if err != nil && !errors.Is(err, db.ErrCareerSheetNotFound) {
		writeError(w, http.StatusInternalServerError, "failed to delete career sheet")
		return
	}
	// Deleting an already-absent sheet is not an error: callers only care
	// that no sheet exists afterward.
	w.WriteHeader(http.StatusNoContent)
}
