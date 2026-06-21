package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gobaas/db"
	"gobaas/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("gobaas_super_secret_key_change_me")

// UserIDKey adalah typed context key untuk user_id — menghindari collision dengan package lain
type AuthContextKey string

const UserIDKey AuthContextKey = "user_id"

func init() {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = []byte(secret)
	} else {
		log.Println("⚠️  PERINGATAN: JWT_SECRET tidak di-set! Menggunakan secret default. JANGAN gunakan di production!")
	}
}

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func RegisterAuthRoutes(r chi.Router) {
	// Group rute auth yang dilindungi oleh API Key middleware
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(middleware.ApiKeyMiddleware)
		r.Post("/signup", handleSignup)
		r.Post("/login", handleLogin)
	})
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	projectID, ok := r.Context().Value(middleware.ProjectIDKey).(string)
	if !ok {
		http.Error(w, "Project ID tidak ditemukan", http.StatusInternalServerError)
		return
	}

	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, "Email dan password dibutuhkan", http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Gagal memproses password", http.StatusInternalServerError)
		return
	}

	// Simpan ke database
	query := `INSERT INTO users (project_id, email, password_hash) VALUES ($1, $2, $3)`
	_, err = db.DB.Exec(query, projectID, req.Email, string(hashedPassword))
	if err != nil {
		// Asumsi duplicate email di project_id yang sama
		http.Error(w, "Email sudah terdaftar di project ini", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User berhasil terdaftar",
		"email":   req.Email,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	projectID, ok := r.Context().Value(middleware.ProjectIDKey).(string)
	if !ok {
		http.Error(w, "Project ID tidak ditemukan", http.StatusInternalServerError)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, "Email dan password dibutuhkan", http.StatusBadRequest)
		return
	}

	var userID string
	var passwordHash string
	query := `SELECT id, password_hash FROM users WHERE project_id = $1 AND email = $2`
	err := db.DB.QueryRow(query, projectID, req.Email).Scan(&userID, &passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Email atau password salah", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Verifikasi password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Email atau password salah", http.StatusUnauthorized)
		return
	}

	// Buat JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    userID,
		"project_id": projectID,
		"email":      req.Email,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Gagal membuat token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": tokenString,
		"token_type":   "Bearer",
	})
}

// UserAuthMiddleware memvalidasi JWT token jika client mengakses data dengan autentikasi user
func UserAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r) // Biarkan lolos jika endpoint bersifat publik (bisa difilter di handler)
			return
		}

		// Toleran terhadap single/double Bearer prefix (e.g. "Bearer Bearer <token>" dari Swagger UI)
		tokenString := authHeader
		for {
			tokenString = strings.TrimSpace(tokenString)
			if len(tokenString) > 7 && strings.EqualFold(tokenString[:7], "bearer ") {
				tokenString = tokenString[7:]
			} else {
				break
			}
		}
		tokenString = strings.TrimSpace(tokenString)

		if tokenString == "" {
			http.Error(w, "Format Authorization tidak valid", http.StatusUnauthorized)
			return
		}
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Token tidak valid atau kedaluwarsa", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Token klaim tidak valid", http.StatusUnauthorized)
			return
		}

		// Masukkan user_id ke context jika valid (menggunakan typed key)
		ctx := context.WithValue(r.Context(), UserIDKey, claims["user_id"])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
