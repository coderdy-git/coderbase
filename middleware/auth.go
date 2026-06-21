package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"gobaas/db"
)

type contextKey string

const ProjectIDKey contextKey = "project_id"

// IsValidAdminSession memvalidasi session cookie dari dashboard
func IsValidAdminSession(r *http.Request) bool {
	cookie, err := r.Cookie("coderbase_session")
	if err != nil {
		return false
	}

	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin123"
	}

	h := hmac.New(sha256.New, []byte("coderbase_session_secret_key_v1"))
	h.Write([]byte(adminPass))
	expected := fmt.Sprintf("cb_sess_%x", h.Sum(nil))

	return cookie.Value == expected
}

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

// CorsMiddleware menangani Cross-Origin Resource Sharing (CORS) agar frontend browser dapat memanggil API.
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
		origin := r.Header.Get("Origin")

		// Tentukan origin yang diizinkan
		if allowedOrigins == "" || allowedOrigins == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// Cek apakah origin masuk dalam daftar allowedOrigins
			matched := false
			for _, allowed := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(allowed) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					matched = true
					break
				}
			}
			if !matched && origin != "" {
				// Origin tidak diizinkan, kembalikan 403 atau biarkan browser memblokirnya
				http.Error(w, "CORS: Origin tidak diizinkan", http.StatusForbidden)
				return
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-API-Key, X-Admin-Key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Tangani preflight request OPTIONS
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
