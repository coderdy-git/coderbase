package policy

import (
	"database/sql"
	"errors"
	"fmt"

	"gobaas/db"

	"github.com/google/uuid"
)

type Policy struct {
	ID         string `json:"id"`
	TableID    string `json:"table_id"`
	Action     string `json:"action"`     // SELECT, INSERT, UPDATE, DELETE
	Role       string `json:"role"`       // anon, authenticated
	Expression string `json:"expression"` // true, auth.uid() = user_id
}

var ErrAccessDenied = errors.New("akses ditolak oleh policy keamanan (RLS)")

// CreatePolicy menyimpan aturan policy baru di database metadata
func CreatePolicy(tableID, action, role, expression string) (string, error) {
	// Validasi input sederhana
	if action != "SELECT" && action != "INSERT" && action != "UPDATE" && action != "DELETE" {
		return "", errors.New("action tidak valid. Harus SELECT, INSERT, UPDATE, atau DELETE")
	}
	if role != "anon" && role != "authenticated" {
		return "", errors.New("role tidak valid. Harus 'anon' atau 'authenticated'")
	}
	if expression != "true" && expression != "auth.uid() = user_id" {
		return "", errors.New("expression tidak didukung. Gunakan 'true' atau 'auth.uid() = user_id'")
	}

	policyID := uuid.New().String()
	query := `INSERT INTO policies (id, table_id, action, role, expression) 
	          VALUES ($1, $2, $3, $4, $5) 
	          ON CONFLICT(table_id, action, role) 
	          DO UPDATE SET expression = EXCLUDED.expression`
	
	_, err := db.DB.Exec(query, policyID, tableID, action, role, expression)
	if err != nil {
		return "", err
	}

	return policyID, nil
}

// EvaluatePolicy memeriksa policy untuk suatu tabel dan mengembalikan klausa SQL tambahan (filter)
// Jika tidak ada policy yang mengizinkan, return error.
// Mengembalikan: (sqlFilter string, isAllowed bool, err error)
func EvaluatePolicy(projectID, tableName, action, userID string) (string, bool, error) {
	// Dapatkan table_id terlebih dahulu
	var tableID string
	err := db.DB.QueryRow("SELECT id FROM tables WHERE project_id = $1 AND name = $2", projectID, tableName).Scan(&tableID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, fmt.Errorf("tabel '%s' tidak ditemukan", tableName)
		}
		return "", false, err
	}

	// Cek apakah ada policy yang terdaftar untuk tabel ini dan aksi ini
	// Jika tidak ada policy sama sekali, secara default kita izinkan (RLS off) agar memudahkan development di awal.
	// Jika ada minimal 1 policy, maka RLS aktif (hanya mengizinkan apa yang dideklarasikan).
	var policyCount int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM policies WHERE table_id = $1 AND action = $2", tableID, action).Scan(&policyCount)
	if err != nil {
		return "", false, err
	}

	if policyCount == 0 {
		return "", true, nil // RLS tidak aktif untuk aksi ini pada tabel ini
	}

	// Cari policy yang cocok dengan role request saat ini
	// Tentukan role berdasarkan keberadaan userID (JWT token)
	role := "anon"
	if userID != "" {
		role = "authenticated"
	}

	var expression string
	query := `SELECT expression FROM policies WHERE table_id = $1 AND action = $2 AND role = $3`
	err = db.DB.QueryRow(query, tableID, action, role).Scan(&expression)
	if err != nil {
		if err == sql.ErrNoRows {
			// Tidak ada policy yang cocok untuk role ini (misal request anon tetapi policy hanya ada untuk authenticated)
			return "", false, ErrAccessDenied
		}
		return "", false, err
	}

	// Evaluasi ekspresi policy
	if expression == "true" {
		return "", true, nil // Diizinkan tanpa filter tambahan
	}

	if expression == "auth.uid() = user_id" {
		if userID == "" {
			return "", false, ErrAccessDenied // Aksi membutuhkan user terautentikasi
		}
		// Return filter sql tambahan: user_id = 'userID'
		return fmt.Sprintf("user_id = '%s'", userID), true, nil
	}

	return "", false, ErrAccessDenied
}
