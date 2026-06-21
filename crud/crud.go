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
	val := r.Context().Value("user_id")
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

func getValidTableAndColumns(projectID, tableName string) (string, map[string]bool, error) {
	var tableID string
	err := db.DB.QueryRow("SELECT id FROM tables WHERE project_id = $1 AND name = $2", projectID, tableName).Scan(&tableID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil, fmt.Errorf("tabel '%s' tidak ditemukan", tableName)
		}
		return "", nil, err
	}

	rows, err := db.DB.Query("SELECT name FROM columns WHERE table_id = $1", tableID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	validCols := map[string]bool{
		"id":         true,
		"project_id": true,
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
	return physicalTable, validCols, nil
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	projectID := r.Context().Value(middleware.ProjectIDKey).(string)
	tableName := chi.URLParam(r, "table_name")
	userID := getUserIDFromContext(r)

	physicalTable, validCols, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	sqlFilter, allowed, err := policy.EvaluatePolicy(projectID, tableName, "SELECT", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	queryStr := fmt.Sprintf("SELECT * FROM %s WHERE project_id = $1", physicalTable)
	if sqlFilter != "" {
		queryStr += " AND " + sqlFilter
	}

	rows, err := db.DB.Query(queryStr, projectID)
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

	physicalTable, validCols, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	sqlFilter, allowed, err := policy.EvaluatePolicy(projectID, tableName, "INSERT", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON request tidak valid", http.StatusBadRequest)
		return
	}

	rowID := uuid.New().String()

	columns := []string{"id", "project_id"}
	placeholders := []string{"$1", "$2"}
	values := []interface{}{rowID, projectID}

	// Jika policy RLS membatasi kepemilikan data (Owner Policy), paksa isi kolom user_id
	if sqlFilter != "" && strings.Contains(sqlFilter, "user_id =") {
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

	physicalTable, validCols, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	sqlFilter, allowed, err := policy.EvaluatePolicy(projectID, tableName, "UPDATE", userID)
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
	if sqlFilter != "" {
		queryStr += " AND " + sqlFilter
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

	physicalTable, _, err := getValidTableAndColumns(projectID, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Evaluasi RLS Policy
	sqlFilter, allowed, err := policy.EvaluatePolicy(projectID, tableName, "DELETE", userID)
	if err != nil || !allowed {
		http.Error(w, "Akses ditolak oleh RLS Policy", http.StatusForbidden)
		return
	}

	queryStr := fmt.Sprintf("DELETE FROM %s WHERE project_id = $1 AND id = $2", physicalTable)
	if sqlFilter != "" {
		queryStr += " AND " + sqlFilter
	}

	res, err := db.DB.Exec(queryStr, projectID, id)
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
