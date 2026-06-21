package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gobaas/crud"
	"gobaas/db"
	"gobaas/policy"
	"gobaas/schema"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func handleAPIGetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"db_type":       "PostgreSQL",
	})
}

func handleAPIGetStats(w http.ResponseWriter, r *http.Request) {
	var stats StatsInfo
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM projects").Scan(&stats.Projects)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM tables").Scan(&stats.Tables)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.Users)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func handleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	logRows, err := db.DB.Query("SELECT method, path, status, latency_ms, error_message, created_at FROM logs ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer logRows.Close()

	logs := []LogItem{}
	for logRows.Next() {
		var l LogItem
		if err := logRows.Scan(&l.Method, &l.Path, &l.Status, &l.Latency, &l.ErrorMessage, &l.CreatedAt); err == nil {
			logs = append(logs, l)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func handleAPIGetProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, name, api_key, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt); err == nil {
			projects = append(projects, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func handleAPICreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Parameter 'name' dibutuhkan", http.StatusBadRequest)
		return
	}

	projectID, apiKey, err := schema.CreateProject(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"project_id": projectID,
		"api_key":    apiKey,
		"name":       req.Name,
	})
}

func handleAPIGetProjectDetail(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	var p Project
	err := db.DB.QueryRow("SELECT id, name, api_key, created_at FROM projects WHERE id = $1", projectID).Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt)
	if err != nil {
		http.Error(w, "Project tidak ditemukan", http.StatusNotFound)
		return
	}

	rows, err := db.DB.Query("SELECT id, project_id, name, created_at FROM tables WHERE project_id = $1 ORDER BY name ASC", projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tables := []Table{}
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.CreatedAt); err == nil {
			tables = append(tables, t)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project": p,
		"tables":  tables,
	})
}

func handleAPICreateTable(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Parameter 'name' dibutuhkan", http.StatusBadRequest)
		return
	}

	tableID, err := schema.CreateTable(projectID, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"table_id":   tableID,
		"name":       req.Name,
		"project_id": projectID,
	})
}

func handleAPIDeleteTable(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")

	err := schema.DropTable(projectID, tableID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Tabel berhasil dihapus",
	})
}

func handleAPIGetTableDetail(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")

	var t Table
	err := db.DB.QueryRow("SELECT id, project_id, name FROM tables WHERE id = $1 AND project_id = $2", tableID, projectID).Scan(&t.ID, &t.ProjectID, &t.Name)
	if err != nil {
		http.Error(w, "Tabel tidak ditemukan", http.StatusNotFound)
		return
	}

	rows, err := db.DB.Query("SELECT id, table_id, name, type, is_nullable FROM columns WHERE table_id = $1 ORDER BY name ASC", tableID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	columns := []Column{}
	for rows.Next() {
		var col Column
		if err := rows.Scan(&col.ID, &col.TableID, &col.Name, &col.Type, &col.IsNullable); err == nil {
			columns = append(columns, col)
		}
	}

	pRows, err := db.DB.Query("SELECT id, table_id, action, role, expression FROM policies WHERE table_id = $1 ORDER BY action ASC", tableID)
	policies := []policy.Policy{}
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var pol policy.Policy
			if err := pRows.Scan(&pol.ID, &pol.TableID, &pol.Action, &pol.Role, &pol.Expression); err == nil {
				policies = append(policies, pol)
			}
		}
	}

	physicalTable := schema.FormatPhysicalTableName(projectID, t.Name)
	dbRows := []map[string]interface{}{}

	queryStr := fmt.Sprintf("SELECT * FROM %s WHERE project_id = $1 ORDER BY created_at DESC LIMIT 100", physicalTable)
	dataRows, err := db.DB.Query(queryStr, projectID)
	if err == nil {
		defer dataRows.Close()
		colNames, _ := dataRows.Columns()
		for dataRows.Next() {
			cols := make([]interface{}, len(colNames))
			colPtrs := make([]interface{}, len(colNames))
			for i := range cols {
				colPtrs[i] = &cols[i]
			}
			if err := dataRows.Scan(colPtrs...); err == nil {
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
				dbRows = append(dbRows, rowMap)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table":    t,
		"columns":  columns,
		"policies": policies,
		"rows":     dbRows,
	})
}

func handleAPIAddColumn(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")

	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		IsNullable bool   `json:"is_nullable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Type == "" {
		http.Error(w, "Parameter 'name' dan 'type' dibutuhkan", http.StatusBadRequest)
		return
	}

	columnID, err := schema.AddColumn(projectID, tableID, req.Name, req.Type, req.IsNullable)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"column_id": columnID,
		"name":      req.Name,
		"type":      req.Type,
	})
}

func handleAPIDeleteColumn(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	columnID := chi.URLParam(r, "column_id")

	err := schema.DropColumn(projectID, tableID, columnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Kolom berhasil dihapus",
	})
}

func handleAPIDeleteRow(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	rowID := chi.URLParam(r, "row_id")

	var tableName string
	err := db.DB.QueryRow("SELECT name FROM tables WHERE id = $1 AND project_id = $2", tableID, projectID).Scan(&tableName)
	if err != nil {
		http.Error(w, "Tabel tidak ditemukan", http.StatusNotFound)
		return
	}

	physicalTable := schema.FormatPhysicalTableName(projectID, tableName)
	_, err = db.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE project_id = $1 AND id = $2", physicalTable), projectID, rowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Data berhasil dihapus",
	})
}

func handleAPIGetUsers(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	rows, err := db.DB.Query("SELECT id, project_id, email, created_at FROM users WHERE project_id = $1 ORDER BY created_at DESC", projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.ProjectID, &u.Email, &u.CreatedAt); err == nil {
			users = append(users, u)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func handleAPICreateUser(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, "Parameter 'email' dan 'password' dibutuhkan", http.StatusBadRequest)
		return
	}

	// Enkripsi password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	query := `INSERT INTO users (id, project_id, email, password_hash) VALUES ($1, $2, $3, $4)`
	_, err = db.DB.Exec(query, userID, projectID, req.Email, string(hashed))
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal menyimpan user: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":         userID,
		"project_id": projectID,
		"email":      req.Email,
	})
}

func handleAPICreatePolicy(w http.ResponseWriter, r *http.Request) {
	tableID := chi.URLParam(r, "table_id")

	var req struct {
		Action     string `json:"action"`
		Role       string `json:"role"`
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Action == "" || req.Role == "" || req.Expression == "" {
		http.Error(w, "Parameter 'action', 'role', dan 'expression' dibutuhkan", http.StatusBadRequest)
		return
	}

	policyID := uuid.New().String()
	query := `INSERT INTO policies (id, table_id, action, role, expression) VALUES ($1, $2, $3, $4, $5)`
	_, err := db.DB.Exec(query, policyID, tableID, req.Action, req.Role, req.Expression)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal menyimpan policy: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":         policyID,
		"table_id":   tableID,
		"action":     req.Action,
		"role":       req.Role,
		"expression": req.Expression,
	})
}

func handleAPIDeletePolicy(w http.ResponseWriter, r *http.Request) {
	tableID := chi.URLParam(r, "table_id")
	var req struct {
		PolicyID string `json:"policy_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PolicyID == "" {
		http.Error(w, "Parameter 'policy_id' dibutuhkan", http.StatusBadRequest)
		return
	}

	query := `DELETE FROM policies WHERE id = $1 AND table_id = $2`
	_, err := db.DB.Exec(query, req.PolicyID, tableID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Policy berhasil dihapus",
	})
}

func handleAPIImportJSON(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")

	var req struct {
		JSONData string `json:"json_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JSONData == "" {
		http.Error(w, "Parameter 'json_data' dibutuhkan", http.StatusBadRequest)
		return
	}

	var t Table
	err := db.DB.QueryRow("SELECT id, project_id, name FROM tables WHERE id = $1 AND project_id = $2", tableID, projectID).Scan(&t.ID, &t.ProjectID, &t.Name)
	if err != nil {
		http.Error(w, "Tabel tidak ditemukan", http.StatusNotFound)
		return
	}

	var rowsArray []map[string]interface{}
	if err := json.Unmarshal([]byte(req.JSONData), &rowsArray); err != nil {
		var singleRow map[string]interface{}
		if err2 := json.Unmarshal([]byte(req.JSONData), &singleRow); err2 == nil {
			rowsArray = append(rowsArray, singleRow)
		} else {
			http.Error(w, "Format JSON tidak valid. Harus berupa Object atau Array of Objects.", http.StatusBadRequest)
			return
		}
	}

	colRows, err := db.DB.Query("SELECT name FROM columns WHERE table_id = $1", tableID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer colRows.Close()

	validCols := map[string]bool{"id": true, "project_id": true, "user_id": true, "created_at": true, "updated_at": true}
	for colRows.Next() {
		var colName string
		if err := colRows.Scan(&colName); err == nil {
			validCols[colName] = true
		}
	}

	physicalTable := schema.FormatPhysicalTableName(projectID, t.Name)

	tx, err := db.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	importedCount := 0
	for _, input := range rowsArray {
		// Siapkan normalizedInput untuk cek duplikat
		normalizedInput := make(map[string]interface{})
		for col, val := range input {
			normalizedCol := strings.ReplaceAll(col, " ", "_")
			matchedCol := ""
			for validColName := range validCols {
				if strings.EqualFold(validColName, normalizedCol) {
					matchedCol = validColName
					break
				}
			}
			if matchedCol != "" {
				if str, ok := val.(string); ok {
					val = strings.ToLower(str)
				}
				normalizedInput[matchedCol] = val
			}
		}

		// Panggil IsDuplicateRecord untuk validasi data duplikat
		isDup, err := crud.IsDuplicateRecord(physicalTable, validCols, projectID, normalizedInput)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal memvalidasi data duplikat saat import: %v", err), http.StatusInternalServerError)
			return
		}
		if isDup {
			continue
		}

		rowID := uuid.New().String()
		columns := []string{"id", "project_id"}
		placeholders := []string{"$1", "$2"}
		values := []interface{}{rowID, projectID}

		paramIndex := 3
		for col, val := range normalizedInput {
			if col == "id" || col == "project_id" || col == "user_id" || col == "created_at" || col == "updated_at" {
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
		_, err = tx.Exec(queryStr, values...)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal menyimpan baris: %v", err), http.StatusBadRequest)
			return
		}
		importedCount++
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Import berhasil",
		"imported": importedCount,
	})
}
