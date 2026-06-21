package schema

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type CreateProjectRequest struct {
	Name string `json:"name"`
}

type CreateTableRequest struct {
	Name string `json:"name"`
}

type AddColumnRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsNullable bool   `json:"is_nullable"`
}

func RegisterSchemaRoutes(r chi.Router) {
	r.Route("/api/projects", func(r chi.Router) {
		r.Post("/", handleCreateProject)
		r.Post("/{project_id}/tables", handleCreateTable)
		r.Post("/{project_id}/tables/{table_id}/columns", handleAddColumn)
		r.Delete("/{project_id}/tables/{table_id}/columns/{column_id}", handleDropColumn)
	})
}

func handleDropColumn(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	columnID := chi.URLParam(r, "column_id")

	if projectID == "" || tableID == "" || columnID == "" {
		http.Error(w, "project_id, table_id, dan column_id dibutuhkan", http.StatusBadRequest)
		return
	}

	err := DropColumn(projectID, tableID, columnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Kolom berhasil dihapus",
	})
}

func handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Request tidak valid. Parameter 'name' dibutuhkan.", http.StatusBadRequest)
		return
	}

	projectID, apiKey, err := CreateProject(req.Name)
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

func handleCreateTable(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	if projectID == "" {
		http.Error(w, "project_id dibutuhkan", http.StatusBadRequest)
		return
	}

	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Request tidak valid. Parameter 'name' dibutuhkan.", http.StatusBadRequest)
		return
	}

	tableID, err := CreateTable(projectID, req.Name)
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

func handleAddColumn(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	if projectID == "" || tableID == "" {
		http.Error(w, "project_id dan table_id dibutuhkan", http.StatusBadRequest)
		return
	}

	var req AddColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Type == "" {
		http.Error(w, "Request tidak valid. Parameter 'name' dan 'type' dibutuhkan.", http.StatusBadRequest)
		return
	}

	columnID, err := AddColumn(projectID, tableID, req.Name, req.Type, req.IsNullable)
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
