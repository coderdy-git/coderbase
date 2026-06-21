package crud

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gobaas/auth"
	"gobaas/db"
	"gobaas/middleware"
	"gobaas/policy"
	"gobaas/realtime"
	"gobaas/schema"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func RegisterCRUDRoutes(r chi.Router) {
	r.Route("/api/v1/tables/{table_name}", func(r chi.Router) {
		r.Use(middleware.ApiKeyMiddleware)
		r.Use(auth.UserAuthMiddleware) // Parse JWT user_id jika ada
		r.Get("/", handleGet)
		r.Post("/", handleInsert)
		r.Patch("/{id}", handleUpdate)
		r.Delete("/{id}", handleDelete)
	})
}

// Mengambil userID dari context secara aman
func getUserIDFromContext(r *http.Request) string {
	val := r.Context().Value(auth.UserIDKey)
	if val == nil {
		return ""
	}
	userID, ok := val.(string)
	if !ok {
		return ""
	}
	return userID
}

func getOneRecord(physicalTable string, validCols map[string]bool, projectID, id string) (map[string]interface{}, error) {
	queryStr := fmt.Sprintf("SELECT * FROM %s WHERE project_id = $1 AND id = $2 LIMIT 1", physicalTable)
	rows, err := db.DB.Query(queryStr, projectID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if rows.Next() {
		cols := make([]interface{}, len(colNames))
		colPtrs := make([]interface{}, len(colNames))
		for i := range cols {
			colPtrs[i] = &cols[i]
		}

		if err := rows.Scan(colPtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, colName := range colNames {
			val := cols[i]
			if b, ok := val.([]byte); ok {
				var jsonVal interface{}
				if err := json.Unmarshal(b, &jsonVal); err == nil {
					rowMap[colName] = jsonVal
				} else {
					rowMap[colName] = string(b)
				}
			} else {
				rowMap[colName] = val
			}
		}

		filteredRow := make(map[string]interface{})
		for k, v := range rowMap {
			if validCols[k] {
				filteredRow[k] = v
			}
		}
		return filteredRow, nil
	}

	return nil, sql.ErrNoRows
}

func getValidTableAndColumns(projectID, tableName string) (string, string, map[string]bool, error) {
	var tableID string
	err := db.DB.QueryRow("SELECT id FROM tables WHERE project_id = $1 AND name = $2", projectID, tableName).Scan(&tableID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil, fmt.Errorf("tabel '%s' tidak ditemukan", tableName)
		}
		return "", "", nil, err
	}

	rows, err := db.DB.Query("SELECT name FROM columns WHERE table_id = $1", tableID)
	if err != nil {
		return "", "", nil, err
	}
	defer rows.Close()

	validCols := map[string]bool{
		"id":         true,
		"project_id": true,
		"user_id":    true,
		"created_at": true,
		"updated_at": true,
	}

	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err == nil {
			validCols[colName] = true
		}
	}

	physicalTable := schema.FormatPhysicalTableName(projectID, tableName)
	return tableID, physicalTable, validCols, nil
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	projectID := r.Context().Value(middleware.ProjectIDKey).(string)
	tableName := chi.URLParam(r, "table_name")
	userID := getUserIDFromContext(r)

	_, physicalTable, validCols, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	filterCol, filterVal, allowed, err := policy.EvaluatePolicy(projectID, tableName, "SELECT", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	queryStr := fmt.Sprintf("SELECT * FROM %s WHERE project_id = $1", physicalTable)
	queryArgs := []interface{}{projectID}
	if filterCol != "" {
		queryStr += fmt.Sprintf(" AND %s = $2", filterCol)
		queryArgs = append(queryArgs, filterVal)
	}

	rows, err := db.DB.Query(queryStr, queryArgs...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal mengambil data: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	results := []map[string]interface{}{}

	for rows.Next() {
		cols := make([]interface{}, len(colNames))
		colPtrs := make([]interface{}, len(colNames))
		for i := range cols {
			colPtrs[i] = &cols[i]
		}

		if err := rows.Scan(colPtrs...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		rowMap := make(map[string]interface{})
		for i, colName := range colNames {
			val := cols[i]
			if b, ok := val.([]byte); ok {
				var jsonVal interface{}
				if err := json.Unmarshal(b, &jsonVal); err == nil {
					rowMap[colName] = jsonVal
				} else {
					rowMap[colName] = string(b)
				}
			} else {
				rowMap[colName] = val
			}
		}

		filteredRow := make(map[string]interface{})
		for k, v := range rowMap {
			if validCols[k] {
				filteredRow[k] = v
			}
		}

		results = append(results, filteredRow)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func handleInsert(w http.ResponseWriter, r *http.Request) {
	projectID := r.Context().Value(middleware.ProjectIDKey).(string)
	tableName := chi.URLParam(r, "table_name")
	userID := getUserIDFromContext(r)

	tableID, physicalTable, validCols, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	filterCol, _, allowed, err := policy.EvaluatePolicy(projectID, tableName, "INSERT", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON request tidak valid", http.StatusBadRequest)
		return
	}

	// Ubah semua input string menjadi huruf kecil
	for col, val := range input {
		if str, ok := val.(string); ok {
			input[col] = strings.ToLower(str)
		}
	}

	// Deteksi dan buat kolom baru secara dinamis berdasarkan input data pertama/baru
	hasNewCols := false
	for col, val := range input {
		if !schema.IsSafeName(col) {
			continue // Abaikan kolom dengan nama tidak valid
		}

		if !validCols[col] && col != "id" && col != "project_id" && col != "user_id" && col != "created_at" && col != "updated_at" {
			// Tentukan tipe data
			colType := "text"
			switch v := val.(type) {
			case bool:
				colType = "boolean"
			case float64:
				if v == float64(int64(v)) {
					colType = "integer"
				} else {
					colType = "float"
				}
			case string:
				colType = "text"
			case map[string]interface{}, []interface{}:
				colType = "jsonb"
			}

			// Buat kolom di database & metadata secara dinamis!
			_, err := schema.AddColumn(projectID, tableID, col, colType, true)
			if err != nil {
				http.Error(w, fmt.Sprintf("Gagal membuat kolom baru otomatis '%s': %v", col, err), http.StatusInternalServerError)
				return
			}
			validCols[col] = true
			hasNewCols = true
		}
	}

	// Jika ada kolom baru, refresh validCols agar sinkron
	if hasNewCols {
		_, _, validCols, _ = getValidTableAndColumns(projectID, tableName)
	}

	rowID := uuid.New().String()

	columns := []string{"id", "project_id"}
	placeholders := []string{"$1", "$2"}
	values := []interface{}{rowID, projectID}

	// Jika policy RLS membatasi kepemilikan data (Owner Policy), paksa isi kolom user_id
	if filterCol == "user_id" {
		// Pastikan kolom user_id terdaftar di tabel fisik
		if validCols["user_id"] {
			input["user_id"] = userID
		} else {
			http.Error(w, "Tabel harus memiliki kolom 'user_id' untuk menerapkan Owner Policy RLS", http.StatusBadRequest)
			return
		}
	}

	paramIndex := 3
	for col, val := range input {
		if !validCols[col] || col == "id" || col == "project_id" || col == "created_at" || col == "updated_at" {
			continue
		}

		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", paramIndex))
		
		switch v := val.(type) {
		case map[string]interface{}, []interface{}:
			jsonBytes, _ := json.Marshal(v)
			values = append(values, string(jsonBytes))
		default:
			values = append(values, val)
		}
		
		paramIndex++
	}

	// Cek validasi double data (data ganda/duplikat)
	isDup, err := IsDuplicateRecord(physicalTable, validCols, projectID, input)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal memvalidasi data duplikat: %v", err), http.StatusInternalServerError)
		return
	}
	if isDup {
		http.Error(w, "Data duplikat terdeteksi. Baris dengan nilai kolom yang sama sudah ada di tabel ini.", http.StatusBadRequest)
		return
	}

	queryStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		physicalTable,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = db.DB.Exec(queryStr, values...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal menyimpan data: %v", err), http.StatusBadRequest)
		return
	}

	record, err := getOneRecord(physicalTable, validCols, projectID, rowID)
	if err == nil {
		realtime.GlobalHub.Broadcast(projectID, tableName, "INSERT", record)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(record)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	projectID := r.Context().Value(middleware.ProjectIDKey).(string)
	tableName := chi.URLParam(r, "table_name")
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r)

	_, physicalTable, validCols, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	filterCol, filterVal, allowed, err := policy.EvaluatePolicy(projectID, tableName, "UPDATE", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON request tidak valid", http.StatusBadRequest)
		return
	}

	setClauses := []string{"updated_at = CURRENT_TIMESTAMP"}
	values := []interface{}{projectID, id}

	paramIndex := 3
	for col, val := range input {
		if !validCols[col] || col == "id" || col == "project_id" || col == "created_at" || col == "updated_at" {
			continue
		}

		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, paramIndex))
		
		switch v := val.(type) {
		case map[string]interface{}, []interface{}:
			jsonBytes, _ := json.Marshal(v)
			values = append(values, string(jsonBytes))
		default:
			values = append(values, val)
		}
		
		paramIndex++
	}

	queryStr := fmt.Sprintf(
		"UPDATE %s SET %s WHERE project_id = $1 AND id = $2",
		physicalTable,
		strings.Join(setClauses, ", "),
	)
	if filterCol != "" {
		queryStr += fmt.Sprintf(" AND %s = $%d", filterCol, paramIndex)
		values = append(values, filterVal)
	}

	res, err := db.DB.Exec(queryStr, values...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal memperbarui data: %v", err), http.StatusBadRequest)
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Data tidak ditemukan atau tidak diizinkan oleh RLS Policy", http.StatusNotFound)
		return
	}

	record, err := getOneRecord(physicalTable, validCols, projectID, id)
	if err == nil {
		realtime.GlobalHub.Broadcast(projectID, tableName, "UPDATE", record)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	projectID := r.Context().Value(middleware.ProjectIDKey).(string)
	tableName := chi.URLParam(r, "table_name")
	id := chi.URLParam(r, "id")
	userID := getUserIDFromContext(r)

	_, physicalTable, _, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	filterCol, filterVal, allowed, err := policy.EvaluatePolicy(projectID, tableName, "DELETE", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	queryStr := fmt.Sprintf("DELETE FROM %s WHERE project_id = $1 AND id = $2", physicalTable)
	deleteArgs := []interface{}{projectID, id}
	if filterCol != "" {
		queryStr += fmt.Sprintf(" AND %s = $3", filterCol)
		deleteArgs = append(deleteArgs, filterVal)
	}

	res, err := db.DB.Exec(queryStr, deleteArgs...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Data tidak ditemukan atau tidak diizinkan oleh RLS Policy", http.StatusNotFound)
		return
	}

	realtime.GlobalHub.Broadcast(projectID, tableName, "DELETE", map[string]string{"id": id})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Data berhasil dihapus",
		"id":      id,
	})
}

func IsDuplicateRecord(physicalTable string, validCols map[string]bool, projectID string, input map[string]interface{}) (bool, error) {
	// Temukan kolom-kolom kustom (bukan kolom metadata)
	var customCols []string
	for col := range validCols {
		if col == "id" || col == "project_id" || col == "user_id" || col == "created_at" || col == "updated_at" {
			continue
		}
		customCols = append(customCols, col)
	}

	// Jika tidak ada kolom kustom, anggap bukan duplikat
	if len(customCols) == 0 {
		return false, nil
	}

	queryParts := []string{"SELECT EXISTS(SELECT 1 FROM " + physicalTable + " WHERE project_id = $1"}
	queryArgs := []interface{}{projectID}
	paramIndex := 2

	for _, col := range customCols {
		val, exists := input[col]
		if !exists || val == nil {
			queryParts = append(queryParts, fmt.Sprintf("%s IS NULL", col))
		} else {
			queryParts = append(queryParts, fmt.Sprintf("%s = $%d", col, paramIndex))
			switch v := val.(type) {
			case map[string]interface{}, []interface{}:
				jsonBytes, _ := json.Marshal(v)
				queryArgs = append(queryArgs, string(jsonBytes))
			default:
				queryArgs = append(queryArgs, val)
			}
			paramIndex++
		}
	}

	fullQuery := strings.Join(queryParts, " AND ") + ")"
	
	var isDup bool
	err := db.DB.QueryRow(fullQuery, queryArgs...).Scan(&isDup)
	if err != nil {
		return false, err
	}
	return isDup, nil
}
