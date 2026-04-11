package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		log.Fatal("POSTGRES_URL is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("postgres open error: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("postgres ping error: %v", err)
	}
	DB = db
	log.Println("Connected to PostgreSQL!")
	migrateDB()
}

func migrateDB() {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		username        TEXT PRIMARY KEY,
		password_hash   TEXT NOT NULL,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	ALTER TABLE users ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';

	CREATE TABLE IF NOT EXISTS friend_requests (
		from_username   TEXT NOT NULL,
		to_username     TEXT NOT NULL,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (from_username, to_username)
	);

	CREATE TABLE IF NOT EXISTS friends (
		user_a          TEXT NOT NULL,
		user_b          TEXT NOT NULL,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (user_a, user_b)
	);
	`
	if _, err := DB.Exec(schema); err != nil {
		log.Fatalf("schema migration error: %v", err)
	}
	log.Println("DB schema up to date")
}
