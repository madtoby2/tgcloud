package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/madtoby2/tgcloud/internal/manager"
	"github.com/madtoby2/tgcloud/internal/store"
)

type Handler struct {
	mgr *manager.Manager
	wsh *WSHub
}

func New(mgr *manager.Manager, wsh *WSHub) *Handler {
	return &Handler{mgr: mgr, wsh: wsh}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Get("/accounts", h.listAccounts)
		r.Post("/accounts", h.addAccount)
		r.Delete("/accounts/{id}", h.deleteAccount)
		r.Post("/accounts/{id}/login", h.startLogin)
		r.Post("/accounts/{id}/code", h.submitCode)
		r.Post("/accounts/{id}/password", h.submitPassword)
		r.Get("/operations", h.listOperations)
		r.Post("/operations", h.createOperation)
		r.Post("/operations/{id}/cancel", h.cancelOperation)
		r.Get("/status", h.systemStatus)
	})
	r.Get("/ws", h.wsh.ServeWS)
	return r
}

func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.mgr.ListAccounts()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if accounts == nil {
		accounts = []*store.Account{}
	}
	jsonOK(w, accounts)
}

func (h *Handler) addAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Proxy string `json:"proxy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if req.Phone == "" {
		jsonError(w, "phone required", 400)
		return
	}
	account, err := h.mgr.AddAccount(req.Phone, req.Proxy)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	h.wsh.Broadcast("account_created", account)
	jsonOK(w, account)
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	if err := h.mgr.DeleteAccount(id); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	h.wsh.Broadcast("account_deleted", map[string]int64{"id": id})
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *Handler) startLogin(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	account, err := h.mgr.GetAccount(id)
	if err != nil {
		jsonError(w, "account not found", 404)
		return
	}
	p, err := h.mgr.RequestLogin(account.Phone)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	// Wait for error or done in background
	go func() {
		select {
		case err := <-p.ErrChan:
			h.wsh.Broadcast("login_error", map[string]interface{}{
				"phone": account.Phone,
				"error": err.Error(),
			})
		case <-p.DoneChan:
			h.wsh.Broadcast("login_done", map[string]interface{}{
				"phone": account.Phone,
			})
		}
	}()

	jsonOK(w, map[string]string{"status": "code_sent", "phone": account.Phone})
}

func (h *Handler) submitCode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	account, err := h.mgr.GetAccount(id)
	if err != nil {
		jsonError(w, "account not found", 404)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if err := h.mgr.SubmitCode(account.Phone, req.Code); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonOK(w, map[string]string{"status": "code_submitted"})
}

func (h *Handler) submitPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	account, err := h.mgr.GetAccount(id)
	if err != nil {
		jsonError(w, "account not found", 404)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if err := h.mgr.SubmitPassword(account.Phone, req.Password); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonOK(w, map[string]string{"status": "password_submitted"})
}

func (h *Handler) listOperations(w http.ResponseWriter, r *http.Request) {
	accountID := int64(0)
	if v := r.URL.Query().Get("account_id"); v != "" {
		accountID, _ = strconv.ParseInt(v, 10, 64)
	}
	ops, err := h.mgr.ListOperations(accountID)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if ops == nil {
		ops = []*store.Operation{}
	}
	jsonOK(w, ops)
}

func (h *Handler) createOperation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64           `json:"account_id"`
		Type      string          `json:"type"`
		Params    json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	op, err := h.mgr.CreateOperation(req.AccountID, req.Type, req.Params)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	h.wsh.Broadcast("operation_created", op)
	jsonOK(w, op)
}

func (h *Handler) cancelOperation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	h.mgr.CancelOperation(id)
	jsonOK(w, map[string]string{"status": "cancelled"})
}

func (h *Handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.mgr.ListAccounts()
	online := 0
	for _, a := range accounts {
		if a.Status == "online" {
			online++
		}
	}
	jsonOK(w, map[string]interface{}{
		"total_accounts":  len(accounts),
		"online_accounts": online,
	})
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "data": data})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": msg})
}
