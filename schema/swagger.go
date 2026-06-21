package schema

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gobaas/db"
	"gobaas/middleware"

	"github.com/go-chi/chi/v5"
)

// SwaggerSpecGenerator merender OpenAPI 3.0 JSON spec secara dinamis berdasarkan skema tabel user
func SwaggerSpecGenerator(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	if projectID == "" {
		http.Error(w, "project_id dibutuhkan", http.StatusBadRequest)
		return
	}

	// 1. Cek apakah diakses oleh Admin via dashboard session cookie
	if middleware.IsValidAdminSession(r) {
		// Admin memiliki akses ke seluruh proyek di studio
	} else {
		// 2. Jika bukan admin dashboard, harus menggunakan X-API-Key yang valid
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API Key atau Session dibutuhkan untuk mengakses skema", http.StatusUnauthorized)
			return
		}

		var projectIDFromKey string
		err := db.DB.QueryRow("SELECT id FROM projects WHERE api_key = $1", apiKey).Scan(&projectIDFromKey)
		if err != nil || projectID != projectIDFromKey {
			http.Error(w, "Akses tidak sah untuk proyek ini (X-API-Key tidak valid)", http.StatusUnauthorized)
			return
		}
	}

	var projectName string
	err := db.DB.QueryRow("SELECT name FROM projects WHERE id = $1", projectID).Scan(&projectName)
	if err != nil {
		http.Error(w, "Project tidak ditemukan", http.StatusNotFound)
		return
	}

	// Ambil semua tabel
	tRows, err := db.DB.Query("SELECT id, name FROM tables WHERE project_id = $1", projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tRows.Close()

	paths := make(map[string]interface{})
	components := make(map[string]interface{})
	schemas := make(map[string]interface{})

	for tRows.Next() {
		var tableID, tableName string
		if err := tRows.Scan(&tableID, &tableName); err != nil {
			continue
		}

		// Ambil kolom tabel
		cRows, err := db.DB.Query("SELECT name, type, is_nullable FROM columns WHERE table_id = $1", tableID)
		if err != nil {
			continue
		}

		properties := make(map[string]interface{})
		// Kolom bawaan sistem
		properties["id"] = map[string]string{"type": "string", "format": "uuid"}
		properties["project_id"] = map[string]string{"type": "string", "format": "uuid"}
		properties["created_at"] = map[string]string{"type": "string", "format": "date-time"}
		properties["updated_at"] = map[string]string{"type": "string", "format": "date-time"}

		for cRows.Next() {
			var colName, colType string
			var isNullable bool
			if err := cRows.Scan(&colName, &colType, &isNullable); err == nil {
				swaggerType := "string"
				if colType == "integer" || colType == "int" {
					swaggerType = "integer"
				} else if colType == "boolean" || colType == "bool" {
					swaggerType = "boolean"
				} else if colType == "float" || colType == "double" {
					swaggerType = "number"
				} else if colType == "jsonb" || colType == "json" {
					swaggerType = "object"
				}
				properties[colName] = map[string]string{"type": swaggerType}
			}
		}
		cRows.Close()

		// Definisikan Model Schema
		schemas[tableName] = map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}

		// Parameter header bawaan untuk semua rute API
		headerParams := []interface{}{
			map[string]interface{}{
				"name":        "X-API-Key",
				"in":          "header",
				"required":    true,
				"description": "API Key Proyek (gb_...)",
				"schema":      map[string]string{"type": "string"},
			},
			map[string]interface{}{
				"name":        "Authorization",
				"in":          "header",
				"required":    false,
				"description": "JWT Token (Format: Bearer <token>) - Diperlukan jika RLS aktif",
				"schema":      map[string]string{"type": "string"},
			},
		}

		// Rute GET / POST untuk tabel ini
		tablePath := fmt.Sprintf("/api/v1/tables/%s", tableName)
		paths[tablePath] = map[string]interface{}{
			"get": map[string]interface{}{
				"summary":    fmt.Sprintf("Mendapatkan seluruh record dari tabel %s", tableName),
				"tags":       []string{tableName},
				"parameters": headerParams,
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Berhasil mengambil data list",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"items": map[string]string{
										"$ref": fmt.Sprintf("#/components/schemas/%s", tableName),
									},
								},
							},
						},
					},
				},
			},
			"post": map[string]interface{}{
				"summary":    fmt.Sprintf("Memasukkan record baru ke tabel %s", tableName),
				"tags":       []string{tableName},
				"parameters": headerParams,
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]string{
								"$ref": fmt.Sprintf("#/components/schemas/%s", tableName),
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"201": map[string]interface{}{
						"description": "Berhasil membuat data",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]string{
									"$ref": fmt.Sprintf("#/components/schemas/%s", tableName),
								},
							},
						},
					},
				},
			},
		}

		// Rute PATCH / DELETE untuk tabel ini
		itemPath := fmt.Sprintf("/api/v1/tables/%s/{id}", tableName)
		patchParams := append([]interface{}{
			map[string]interface{}{
				"name":        "id",
				"in":          "path",
				"required":    true,
				"description": "ID data record",
				"schema":      map[string]string{"type": "string"},
			},
		}, headerParams...)

		paths[itemPath] = map[string]interface{}{
			"patch": map[string]interface{}{
				"summary":    fmt.Sprintf("Memperbarui record di tabel %s berdasarkan ID", tableName),
				"tags":       []string{tableName},
				"parameters": patchParams,
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]string{
								"$ref": fmt.Sprintf("#/components/schemas/%s", tableName),
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Berhasil memperbarui data",
					},
				},
			},
			"delete": map[string]interface{}{
				"summary":    fmt.Sprintf("Menghapus record dari tabel %s berdasarkan ID", tableName),
				"tags":       []string{tableName},
				"parameters": patchParams,
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Berhasil menghapus data",
					},
				},
			},
		}
	}

	components["schemas"] = schemas
	// Definisikan autentikasi API Key dan JWT Bearer
	components["securitySchemes"] = map[string]interface{}{
		"ApiKeyAuth": map[string]interface{}{
			"type": "apiKey",
			"in":   "header",
			"name": "X-API-Key",
		},
		"BearerAuth": map[string]interface{}{
			"type":         "http",
			"scheme":       "bearer",
			"bearerFormat": "JWT",
		},
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	swaggerSpec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":       fmt.Sprintf("Coderbase API Docs - %s", projectName),
			"version":     "1.0.0",
			"description": fmt.Sprintf("Dokumentasi REST API dinamis Coderbase BaaS untuk proyek %s.", projectName),
		},
		"servers": []map[string]string{
			{
				"url":         baseURL,
				"description": "Server API Coderbase",
			},
		},
		"paths":      paths,
		"components": components,
		"security": []map[string]interface{}{
			{"ApiKeyAuth": []string{}},
			{"BearerAuth": []string{}},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(swaggerSpec)
}
