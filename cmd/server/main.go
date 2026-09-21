package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/k07g/g5/internal/api"
	"github.com/k07g/g5/internal/auth"
	"github.com/k07g/g5/internal/config"
	"github.com/k07g/g5/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx := context.Background()

	var verifier auth.Verifier
	switch cfg.AuthProvider {
	case config.AuthProviderMemory:
		log.Println("AUTH_PROVIDER=memory: using in-memory auth verifier (local development only, not for production)")
		verifier = auth.NewMemoryVerifier()
	default:
		cognitoVerifier, err := auth.NewCognitoVerifier(ctx, cfg.AWSRegion)
		if err != nil {
			log.Fatalf("failed to init cognito verifier: %v", err)
		}
		verifier = cognitoVerifier
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(ctx, database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	careerSheets := db.NewCareerSheetRepository(database)
	handler := api.NewHandler(careerSheets)
	router := api.NewRouter(handler, api.AuthMiddleware(verifier))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	log.Printf("server listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
