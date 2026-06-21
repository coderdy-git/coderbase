package main

import (
	"log"
	"net/http"
	"os"

	"gobaas/auth"
	"gobaas/crud"
	"gobaas/dashboard"
	"gobaas/db"
	"gobaas/middleware"
	"gobaas/policy"
	"gobaas/realtime"
	"gobaas/schema"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Inisialisasi Database
	db.InitDB()
	defer db.DB.Close()

	// Jalankan Realtime WebSocket Hub
	go realtime.GlobalHub.Run()

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SystemLoggerMiddleware) // Logger sistem GoBaaS

	schema.RegisterSchemaRoutes(r)
	auth.RegisterAuthRoutes(r)
	crud.RegisterCRUDRoutes(r)
	dashboard.RegisterDashboardRoutes(r)
	policy.RegisterPolicyRoutes(r)

	// Swagger API JSON Spec Endpoint
	r.Get("/api/projects/{project_id}/swagger.json", schema.SwaggerSpecGenerator)

	// WebSocket Realtime Endpoint
	r.Get("/api/v1/realtime", realtime.HandleRealtime)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Go BaaS API is running!"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server berjalan di port %s...\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Gagal menjalankan server: %v", err)
	}
}
