package middleware

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

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

// AdminKeyMiddleware melindungi admin API routes (schema, policy management).
// Membutuhkan header X-Admin-Key yang cocok dengan env ADMIN_API_KEY.
func AdminKeyMiddleware(next http.Handler) http.Handler {
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		log.Println("⚠️  PERINGATAN: ADMIN_API_KEY tidak di-set! Admin API routes dilindungi oleh key default.")
		adminKey = "cb_admin_change_me_in_production"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providedKey := r.Header.Get("X-Admin-Key")
		if providedKey == "" {
			http.Error(w, "X-Admin-Key header dibutuhkan untuk akses admin API", http.StatusUnauthorized)
			return
		}

		if providedKey != adminKey {
			http.Error(w, "X-Admin-Key tidak valid", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
