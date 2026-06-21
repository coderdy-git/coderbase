package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL tidak di-set! Proyek ini membutuhkan PostgreSQL.")
	}

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Gagal membuka koneksi PostgreSQL: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Gagal ping PostgreSQL: %v", err)
	}

	log.Println("Berhasil terhubung ke database PostgreSQL!")
	createMetaTables()
}

func createMetaTables() {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
		
		`CREATE TABLE IF NOT EXISTS projects (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL,
			api_key VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			email VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_id, email)
		);`,

		`CREATE TABLE IF NOT EXISTS tables (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_id, name)
		);`,

		`CREATE TABLE IF NOT EXISTS columns (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			table_id UUID NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			is_nullable BOOLEAN DEFAULT TRUE,
			UNIQUE(table_id, name)
		);`,

		`CREATE TABLE IF NOT EXISTS policies (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			table_id UUID NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
			action VARCHAR(20) NOT NULL,
			role VARCHAR(20) NOT NULL,
			expression VARCHAR(255) NOT NULL,
			UNIQUE(table_id, action, role)
		);`,

		`CREATE TABLE IF NOT EXISTS logs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
			method VARCHAR(10) NOT NULL,
			path TEXT NOT NULL,
			status INTEGER NOT NULL,
			ip_address VARCHAR(45) NOT NULL,
			latency_ms INTEGER NOT NULL,
			error_message TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		_, err := DB.Exec(q)
		if err != nil {
			log.Fatalf("Gagal membuat tabel meta: %v\nQuery: %s", err, q)
		}
	}

	// Migrasi otomatis: Tambahkan kolom error_message ke tabel logs jika belum ada di database yang sudah terpasang
	_, err := DB.Exec("ALTER TABLE logs ADD COLUMN IF NOT EXISTS error_message TEXT;")
	if err != nil {
		log.Printf("Gagal menjalankan migrasi logs: %v", err)
	}

	log.Println("Tabel meta berhasil diinisialisasi.")
}
