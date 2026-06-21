package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/glebarez/go-sqlite"
	_ "github.com/lib/pq"
)

var DB *sql.DB
var DBType string // "postgres" atau "sqlite"

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	
	if dsn == "" {
		dsn = "sqlite://gobaas.db"
	}

	var err error
	if len(dsn) >= 9 && dsn[:9] == "sqlite://" {
		DBType = "sqlite"
		dbPath := dsn[9:]
		DB, err = sql.Open("sqlite", dbPath)
		if err != nil {
			log.Fatalf("Gagal membuka database SQLite: %v", err)
		}
		log.Printf("Menggunakan database SQLite lokal di: %s\n", dbPath)
	} else {
		DBType = "postgres"
		DB, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("Gagal membuka koneksi PostgreSQL: %v", err)
		}
	}

	if err = DB.Ping(); err != nil {
		if DBType == "postgres" {
			log.Printf("Gagal ping PostgreSQL: %v. Mencoba fallback ke SQLite lokal...\n", err)
			DBType = "sqlite"
			DB, err = sql.Open("sqlite", "gobaas.db")
			if err != nil || DB.Ping() != nil {
				log.Fatalf("Gagal inisialisasi database fallback: %v", err)
			}
			log.Println("Berhasil menggunakan database fallback SQLite lokal: gobaas.db")
		} else {
			log.Fatalf("Gagal ping database: %v", err)
		}
	} else {
		log.Printf("Berhasil terhubung ke database (%s)!\n", DBType)
	}

	createMetaTables()
}

func createMetaTables() {
	var queries []string

	if DBType == "postgres" {
		queries = []string{
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
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);`,
		}
	} else {
		// DDL untuk SQLite
		queries = []string{
			`CREATE TABLE IF NOT EXISTS projects (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				api_key TEXT UNIQUE NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				email TEXT NOT NULL,
				password_hash TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(project_id, email)
			);`,

			`CREATE TABLE IF NOT EXISTS tables (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				name TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(project_id, name)
			);`,

			`CREATE TABLE IF NOT EXISTS columns (
				id TEXT PRIMARY KEY,
				table_id TEXT NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				is_nullable INTEGER DEFAULT 1,
				UNIQUE(table_id, name)
			);`,

			`CREATE TABLE IF NOT EXISTS policies (
				id TEXT PRIMARY KEY,
				table_id TEXT NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
				action TEXT NOT NULL,
				role TEXT NOT NULL,
				expression TEXT NOT NULL,
				UNIQUE(table_id, action, role)
			);`,

			`CREATE TABLE IF NOT EXISTS logs (
				id TEXT PRIMARY KEY,
				project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
				method TEXT NOT NULL,
				path TEXT NOT NULL,
				status INTEGER NOT NULL,
				ip_address TEXT NOT NULL,
				latency_ms INTEGER NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);`,
		}
	}

	for _, q := range queries {
		_, err := DB.Exec(q)
		if err != nil {
			log.Fatalf("Gagal membuat tabel meta: %v\nQuery: %s", err, q)
		}
	}
	log.Println("Tabel meta berhasil diinisialisasi.")
}
