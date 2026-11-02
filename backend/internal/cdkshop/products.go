package cdkshop

import (
	"encoding/json"
	"net/http"
)

func (s *Server) adminSetProductActive(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Active *bool `json:"active"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body) != nil || body.Active == nil {
		writeError(w, http.StatusBadRequest, "invalid_product")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `UPDATE cdkshop_products SET active=$2,version=version+1 WHERE id=$1 AND deleted_at IS NULL`, r.PathValue("id"), *body.Active)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "product_update_failed")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "product_update_failed")
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "product_not_found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": *body.Active})
}

func (s *Server) adminDeleteProduct(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	// Keep stock and order snapshots so pending payments and historical code
	// retrieval remain valid. UPDATE shares the checkout product row lock.
	result, err := s.db.ExecContext(r.Context(), `UPDATE cdkshop_products SET active=false,deleted_at=NOW(),version=version+1 WHERE id=$1 AND deleted_at IS NULL`, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "product_delete_failed")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "product_delete_failed")
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "product_not_found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
