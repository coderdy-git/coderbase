package middleware

import (
	"net/http"
	"strings"
	"time"

	"gobaas/db"

	"github.com/google/uuid"
)

// responseWriterInterceptor untuk membungkus http.ResponseWriter agar bisa merekam HTTP status code & error body
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (w *responseWriterInterceptor) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriterInterceptor) Write(b []byte) (int, error) {
	if w.statusCode >= 400 {
		w.body = append(w.body, b...)
	}
	return w.ResponseWriter.Write(b)
}

// SystemLoggerMiddleware merekam log transaksi request API BaaS ke database secara async
func SystemLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Bungkus ResponseWriter
		wi := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wi, r)

		latency := time.Since(start).Milliseconds()

		// Ambil project_id dari context jika ada
		var projectID *string
		if val := r.Context().Value(ProjectIDKey); val != nil {
			if pid, ok := val.(string); ok && pid != "" {
				projectID = &pid
			}
		}

		// Ambil IP address sederhana
		ip := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = xff
		}

		// Ambil pesan error jika ada
		var errMsg *string
		if wi.statusCode >= 400 && len(wi.body) > 0 {
			cleanMsg := strings.TrimSpace(string(wi.body))
			if cleanMsg != "" {
				errMsg = &cleanMsg
			}
		}

		go func(pid *string, method, path string, status int, ipAddress string, duration int64, errStr *string) {
			logID := uuid.New().String()
			
			query := `INSERT INTO logs (id, project_id, method, path, status, ip_address, latency_ms, error_message) 
			         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
			_, err := db.DB.Exec(query, logID, pid, method, path, status, ipAddress, duration, errStr)
			
			if err != nil {
				// Cukup cetak ke console jika log gagal disimpan, jangan crash
				println("Gagal menyimpan log transaksi:", err.Error())
			}
		}(projectID, r.Method, r.URL.Path, wi.statusCode, ip, latency, errMsg)
	})
}
