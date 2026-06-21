package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"gobaas/db"
)

type contextKey string

const ProjectIDKey contextKey = "project_id"

// ApiKeyMiddleware memvalidasi X-API-Key dari header dan menyimpan project_id ke context
func ApiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "X-API-Key header dibutuhkan", http.StatusUnauthorized)
			return
		}

		var projectID string
		err := db.DB.QueryRow("SELECT id FROM projects WHERE api_key = $1", apiKey).Scan(&projectID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "X-API-Key tidak valid", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Kesalahan server internal", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), ProjectIDKey, projectID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
