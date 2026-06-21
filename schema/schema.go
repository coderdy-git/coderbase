package schema

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gobaas/db"

	"github.com/google/uuid"
)

var safeNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func IsSafeName(name string) bool {
	return safeNameRegex.MatchString(name) && len(name) < 64
}

func mapDataType(inputType string, dbType string) (string, error) {
	lowerType := strings.ToLower(inputType)
	if dbType == "sqlite" {
		switch lowerType {
		case "text", "string", "jsonb", "json":
			return "TEXT", nil
		case "integer", "int":
			return "INTEGER", nil
		case "boolean", "bool":
			return "INTEGER", nil // SQLite tidak punya boolean tipe khusus, gunakan integer (0/1)
		case "timestamp", "datetime":
			return "DATETIME", nil
		case "float", "double":
			return "REAL", nil
		default:
			return "", fmt.Errorf("tipe data tidak didukung di SQLite: %s", inputType)
		}
	}

	// Postgres mapping
	switch lowerType {
	case "text", "string":
		return "TEXT", nil
	case "integer", "int":
		return "INTEGER", nil
	case "boolean", "bool":
		return "BOOLEAN", nil
	case "timestamp", "datetime":
		return "TIMESTAMP WITH TIME ZONE", nil
	case "jsonb", "json":
		return "JSONB", nil
	case "float", "double":
		return "DOUBLE PRECISION", nil
	default:
		return "", fmt.Errorf("tipe data tidak didukung di Postgres: %s", inputType)
	}
}

func FormatPhysicalTableName(projectID string, tableName string) string {
	cleanID := strings.ReplaceAll(projectID, "-", "_")
	return fmt.Sprintf("p_%s_%s", cleanID, tableName)
}

func CreateProject(name string) (string, string, error) {
	projectID := uuid.New().String()
	apiKey := "gb_" + uuid.New().String()

	query := `INSERT INTO projects (id, name, api_key) VALUES ($1, $2, $3)`
	_, err := db.DB.Exec(query, projectID, name, apiKey)
	if err != nil {
		return "", "", err
	}

	return projectID, apiKey, nil
}

func CreateTable(projectID string, tableName string) (string, error) {
	if !IsSafeName(tableName) {
		return "", errors.New("nama tabel tidak valid")
	}

	var exists bool
	err := db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)", projectID).Scan(&exists)
	if err != nil || !exists {
		return "", errors.New("project tidak ditemukan")
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	tableID := uuid.New().String()
	insertMetaQuery := `INSERT INTO tables (id, project_id, name) VALUES ($1, $2, $3)`
	_, err = tx.Exec(insertMetaQuery, tableID, projectID, tableName)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan metadata: %v", err)
	}

	physicalName := FormatPhysicalTableName(projectID, tableName)
	var createTableDDL string

	if db.DBType == "postgres" {
		createTableDDL = fmt.Sprintf(`
			CREATE TABLE %s (
				id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);
		`, physicalName)
	} else {
		// SQLite
		createTableDDL = fmt.Sprintf(`
			CREATE TABLE %s (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
		`, physicalName)
	}

	_, err = tx.Exec(createTableDDL)
	if err != nil {
		return "", fmt.Errorf("gagal membuat tabel fisik %s: %v", physicalName, err)
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return tableID, nil
}

func AddColumn(projectID string, tableID string, columnName string, columnType string, isNullable bool) (string, error) {
	if !IsSafeName(columnName) {
		return "", errors.New("nama kolom tidak valid")
	}

	pgType, err := mapDataType(columnType, db.DBType)
	if err != nil {
		return "", err
	}

	var tableName string
	err = db.DB.QueryRow("SELECT name FROM tables WHERE id = $1 AND project_id = $2", tableID, projectID).Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("tabel tidak ditemukan")
		}
		return "", err
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	columnID := uuid.New().String()
	insertColumnQuery := `INSERT INTO columns (id, table_id, name, type, is_nullable) VALUES ($1, $2, $3, $4, $5)`
	
	// Untuk SQLite boolean disimpan sebagai integer 0/1
	var isNullableVal interface{} = isNullable
	if db.DBType == "sqlite" {
		if isNullable {
			isNullableVal = 1
		} else {
			isNullableVal = 0
		}
	}

	_, err = tx.Exec(insertColumnQuery, columnID, tableID, columnName, columnType, isNullableVal)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan metadata kolom: %v", err)
	}

	physicalTableName := FormatPhysicalTableName(projectID, tableName)
	nullability := "NULL"
	if !isNullable {
		nullability = "NOT NULL"
		// SQLite punya beberapa batasan dalam ALTER TABLE ADD COLUMN NOT NULL tanpa DEFAULT value.
		// Namun untuk mempermudah, kita biarkan saja NULL / default value jika di SQLite.
		if db.DBType == "sqlite" {
			nullability = "" // Biarkan kosong agar SQLite mau memproses
		}
	}

	alterTableDDL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s %s;", physicalTableName, columnName, pgType, nullability)
	_, err = tx.Exec(alterTableDDL)
	if err != nil {
		return "", fmt.Errorf("gagal memodifikasi tabel fisik %s: %v", physicalTableName, err)
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return columnID, nil
}

func DropColumn(projectID string, tableID string, columnID string) error {
	var columnName string
	var tableName string
	err := db.DB.QueryRow(`
		SELECT c.name, t.name 
		FROM columns c 
		JOIN tables t ON c.table_id = t.id 
		WHERE c.id = $1 AND t.id = $2 AND t.project_id = $3`, 
		columnID, tableID, projectID).Scan(&columnName, &tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("kolom tidak ditemukan")
		}
		return err
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Hapus metadata kolom
	_, err = tx.Exec("DELETE FROM columns WHERE id = $1", columnID)
	if err != nil {
		return err
	}

	// ALTER TABLE DROP COLUMN
	physicalTableName := FormatPhysicalTableName(projectID, tableName)
	alterTableDDL := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", physicalTableName, columnName)
	_, err = tx.Exec(alterTableDDL)
	if err != nil {
		return fmt.Errorf("gagal memodifikasi tabel fisik %s: %v", physicalTableName, err)
	}

	return tx.Commit()
}
