package api

import "net/http"

func NewRouter(h *Handler, authMiddleware func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("GET /career-sheet", authMiddleware(http.HandlerFunc(h.GetCareerSheet)))
	mux.Handle("PUT /career-sheet", authMiddleware(http.HandlerFunc(h.PutCareerSheet)))
	mux.Handle("DELETE /career-sheet", authMiddleware(http.HandlerFunc(h.DeleteCareerSheet)))

	return mux
}
