package cdkshop

import (
	"net/http"
	"strconv"
	"time"
)

type inventoryCode struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	CodeError string    `json:"code_error,omitempty"`
	Status    string    `json:"status"`
	OrderID   string    `json:"order_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) adminInventory(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "available" && status != "reserved" && status != "delivered" {
		writeError(w, http.StatusBadRequest, "invalid_inventory_filter")
		return
	}
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		var err error
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 || page > 1000000 {
			writeError(w, http.StatusBadRequest, "invalid_inventory_page")
			return
		}
	}
	const pageSize = 50
	productID := r.PathValue("id")
	var exists bool
	if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM cdkshop_products WHERE id=$1)`, productID).Scan(&exists); err != nil {
		writeError(w, http.StatusInternalServerError, "inventory_unavailable")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "product_not_found")
		return
	}
	var total int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM cdkshop_cdks WHERE product_id=$1 AND ($2='' OR status=$2)`, productID, status).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "inventory_unavailable")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,encrypted_code,status,COALESCE(order_id,''),created_at FROM cdkshop_cdks WHERE product_id=$1 AND ($2='' OR status=$2) ORDER BY id DESC LIMIT $3 OFFSET $4`, productID, status, pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "inventory_unavailable")
		return
	}
	defer func() { _ = rows.Close() }()
	items := []inventoryCode{}
	for rows.Next() {
		var item inventoryCode
		var encrypted string
		if err = rows.Scan(&item.ID, &encrypted, &item.Status, &item.OrderID, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "inventory_unavailable")
			return
		}
		item.Code, err = s.crypto.Decrypt(encrypted)
		if err != nil {
			item.Code = ""
			item.CodeError = "inventory_decrypt_failed"
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeError(w, http.StatusInternalServerError, "inventory_unavailable")
		return
	}
	// Only this explicit admin request decrypts stock; storage remains encrypted.
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}
