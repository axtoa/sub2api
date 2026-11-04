package cdkshop

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (s *Server) adminUpdateProduct(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var p Product
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&p) != nil {
		writeError(w, http.StatusBadRequest, "invalid_product")
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Spec = strings.TrimSpace(p.Spec)
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 120 || utf8.RuneCountInString(p.Spec) > 120 ||
		utf8.RuneCountInString(p.Description) > 1000 || utf8.RuneCountInString(p.Instructions) > 1000 ||
		p.PriceCents < 1 || p.PriceCents > 10000000 || p.RewardCents < 0 || p.RewardCents > 10000000 || p.Version < 1 {
		writeError(w, http.StatusBadRequest, "invalid_product")
		return
	}
	p.ID = r.PathValue("id")
	// The version check prevents an open editor from overwriting a newer edit
	// or status change. Orders keep their existing price/name/reward snapshots.
	err := s.db.QueryRowContext(r.Context(), `UPDATE cdkshop_products SET name=$2,spec=$3,description=$4,instructions=$5,price_cents=$6,reward_cents=$7,active=$8,version=version+1 WHERE id=$1 AND version=$9 AND deleted_at IS NULL RETURNING version`,
		p.ID, p.Name, p.Spec, p.Description, p.Instructions, p.PriceCents, p.RewardCents, p.Active, p.Version).Scan(&p.Version)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if err = s.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM cdkshop_products WHERE id=$1 AND deleted_at IS NULL)`, p.ID).Scan(&exists); err == nil {
			if exists {
				writeError(w, http.StatusConflict, "product_conflict")
			} else {
				writeError(w, http.StatusNotFound, "product_not_found")
			}
			return
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "product_update_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"version": p.Version})
}

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
	var version int
	err := s.db.QueryRowContext(r.Context(), `UPDATE cdkshop_products SET active=$2,version=version+1 WHERE id=$1 AND deleted_at IS NULL RETURNING version`, r.PathValue("id"), *body.Active).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "product_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "product_update_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": *body.Active, "version": version})
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
