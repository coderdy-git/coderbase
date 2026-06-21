package policy

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type CreatePolicyRequest struct {
	Action     string `json:"action"`     // SELECT, INSERT, UPDATE, DELETE
	Role       string `json:"role"`       // anon, authenticated
	Expression string `json:"expression"` // true, auth.uid() = user_id
}

func RegisterPolicyRoutes(r chi.Router) {
	r.Route("/api/projects/{project_id}/tables/{table_id}/policies", func(r chi.Router) {
		r.Post("/", handleCreatePolicy)
	})
}

func handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	tableID := chi.URLParam(r, "table_id")
	if tableID == "" {
		http.Error(w, "table_id dibutuhkan", http.StatusBadRequest)
		return
	}

	var req CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Action == "" || req.Role == "" || req.Expression == "" {
		http.Error(w, "Request tidak valid. Parameter 'action', 'role', dan 'expression' dibutuhkan.", http.StatusBadRequest)
		return
	}

	policyID, err := CreatePolicy(tableID, req.Action, req.Role, req.Expression)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"policy_id":  policyID,
		"action":     req.Action,
		"role":       req.Role,
		"expression": req.Expression,
	})
}
