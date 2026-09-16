package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

func main() {
	dir := flag.String("dir", "migrations", "directory containing numbered .up.sql migrations")
	flag.Parse()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := dbmigrate.Run(ctx, db, *dir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("migrations applied: %d; already applied: %d", len(result.Applied), len(result.AlreadyApplied))
}
