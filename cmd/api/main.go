package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/app"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	keyPath := os.Getenv("AUTH_PUBLIC_KEY_FILE")
	if dsn == "" || keyPath == "" {
		log.Fatal("DATABASE_URL and AUTH_PUBLIC_KEY_FILE are required")
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		log.Fatal("read configured public key: ", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	handler, err := app.NewHandler(ctx, db, app.Config{
		PublicKeyPEM: key, Issuer: os.Getenv("AUTH_ISSUER"), Audience: os.Getenv("AUTH_AUDIENCE"),
		AgreementVersion: os.Getenv("PROMOTER_AGREEMENT_VERSION"), MigrationsDir: "migrations",
	})
	if err != nil {
		log.Fatal("API configuration or database is not ready: ", err)
	}
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	log.Printf("api listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
