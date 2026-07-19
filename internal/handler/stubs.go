package handler

import "net/http"

// --- System ---

func (h *Handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{
		"version":  "2.0.0",
		"ws_conns": h.WS.ConnCount(),
		"status":   "running",
	})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	if h.AccountSvc == nil {
		jsonOK(w, map[string]int{"total": 0, "online": 0, "banned": 0, "restricted": 0, "frozen": 0, "deactivated": 0, "normal": 0})
		return
	}
	accounts, total, _ := h.AccountSvc.List(AccountFilter{PageSize: 1000})
	online, banned, restricted, frozen, deactivated := 0, 0, 0, 0, 0
	for _, a := range accounts {
		if a.Raw != nil {
			if s, ok := a.Raw["status"].(string); ok && s == "online" {
				online++
			}
			if s, ok := a.Raw["telegram_status"].(string); ok {
				switch s {
				case "banned": banned++
				case "restricted": restricted++
				case "frozen": frozen++
				case "deactivated": deactivated++
				}
			}
		}
	}
	jsonOK(w, map[string]int{
		"total": total, "online": online, "banned": banned,
		"restricted": restricted, "frozen": frozen, "deactivated": deactivated,
		"normal": total - banned - restricted - frozen - deactivated,
	})
}

// --- Placeholder stubs ---

func (h *Handler) checkAccountStatus(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) estimateRegistration(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) changeTwoFA(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) bindRecoveryEmail(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) batchStatusCheck(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) batchDeleteWaste(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

// Import/Export
func (h *Handler) importTelethon(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) importTData(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) exportTelethon(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

// Channels/Groups
func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, []interface{}{})
}

func (h *Handler) syncChannels(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, []interface{}{})
}

func (h *Handler) syncGroups(w http.ResponseWriter, r *http.Request) {
	jsonError(w, "not implemented", 501)
}

// Categories
func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	if h.CategorySvc == nil {
		jsonOK(w, []interface{}{})
		return
	}
	cats, err := h.CategorySvc.List()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, cats)
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		jsonError(w, "name required", 400)
		return
	}
	c, err := h.CategorySvc.Create(req.Name, req.Color)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, c)
}

func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if err := h.CategorySvc.Update(id, req.Name, req.Color); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]string{"status": "updated"})
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if err := h.CategorySvc.Delete(id); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}
