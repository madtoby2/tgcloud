package handler

import (
	"fmt"
	"net/http"

	"github.com/madtoby2/tgcloud/internal/manager"
	"github.com/madtoby2/tgcloud/internal/operator"
	"github.com/madtoby2/tgcloud/internal/store"
)

// Implement AccountService using store
type accountService struct {
	store *store.AccountStore
}

func (s *accountService) List(filter AccountFilter) ([]AccountRow, int, error) {
	sf := store.AccountFilter{
		Status: filter.Status,
		Search: filter.Search,
		Page:   filter.Page,
		PageSize: filter.PageSize,
	}
	if filter.CategoryID > 0 {
		sf.CategoryID = &filter.CategoryID
	}
	accounts, total, err := s.store.List(sf)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]AccountRow, len(accounts))
	for i, a := range accounts {
		rows[i] = AccountRow{ID: a.ID, Phone: a.Phone, Raw: a.ToJSON()}
	}
	return rows, total, nil
}

func (s *accountService) GetByID(id int64) (*AccountRow, error) {
	a, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	return &AccountRow{ID: a.ID, Phone: a.Phone, Raw: a.ToJSON()}, nil
}

func (s *accountService) Create(phone, proxy string) (*AccountRow, error) {
	a, err := s.store.Create(phone, proxy)
	if err != nil {
		return nil, err
	}
	return &AccountRow{ID: a.ID, Phone: a.Phone, Raw: a.ToJSON()}, nil
}

func (s *accountService) Update(id int64, fields map[string]interface{}) error {
	return s.store.Update(id, fields)
}

func (s *accountService) Delete(id int64) error {
	return s.store.Delete(id)
}

func (s *accountService) SetTelegramStatus(id int64, status, detail string) error {
	return s.store.SetTelegramStatus(id, status, detail)
}

func (s *accountService) GetByPhone(phone string) (*AccountRow, error) {
	a, err := s.store.GetByPhone(phone)
	if err != nil {
		return nil, err
	}
	return &AccountRow{ID: a.ID, Phone: a.Phone, Raw: a.ToJSON()}, nil
}

// categoryService adapter
type categoryService struct {
	store *store.CategoryStore
}

func (s *categoryService) List() ([]CategoryRow, error) {
	cats, err := s.store.List()
	if err != nil {
		return nil, err
	}
	rows := make([]CategoryRow, len(cats))
	for i, c := range cats {
		rows[i] = CategoryRow{ID: c.ID, Name: c.Name, Color: c.Color, SortOrder: c.SortOrder}
	}
	return rows, nil
}

func (s *categoryService) GetByID(id int64) (*CategoryRow, error) {
	c, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	return &CategoryRow{ID: c.ID, Name: c.Name, Color: c.Color, SortOrder: c.SortOrder}, nil
}

func (s *categoryService) Create(name, color string) (*CategoryRow, error) {
	c, err := s.store.Create(name, color)
	if err != nil {
		return nil, err
	}
	return &CategoryRow{ID: c.ID, Name: c.Name, Color: c.Color, SortOrder: c.SortOrder}, nil
}

func (s *categoryService) Update(id int64, name, color string) error {
	return s.store.Update(id, name, color)
}

func (s *categoryService) Delete(id int64) error {
	return s.store.Delete(id)
}

// loginManager adapter
type loginManagerAdapter struct {
	mgr *manager.Manager
}

func (l *loginManagerAdapter) AddAccount(phone, proxy string) (int64, error) {
	return l.mgr.AddAccount(phone, proxy)
}

func (l *loginManagerAdapter) RequestLogin(phone string) (*PendingLogin, error) {
	pl, err := l.mgr.RequestLogin(phone)
	if err != nil {
		return nil, err
	}
	return &PendingLogin{
		Phone:    pl.Phone,
		CodeChan: pl.CodeChan,
		PassChan: pl.PassChan,
		ErrChan:  pl.ErrChan,
		DoneChan: pl.DoneChan,
	}, nil
}

func (l *loginManagerAdapter) SubmitCode(phone, code string) error {
	return l.mgr.SubmitCode(phone, code)
}

func (l *loginManagerAdapter) SubmitPassword(phone, password string) error {
	return l.mgr.SubmitPassword(phone, password)
}

// WireServices connects the handler to real implementations
func (h *Handler) WireServices(mgr *manager.Manager, eng *operator.Engine, st *store.Store) {
	h.store = st
	h.engine = eng
	h.mgr = mgr

	accountStore := store.NewAccountStore(st.DB())
	catStore := store.NewCategoryStore(st.DB())

	h.AccountSvc = &accountService{store: accountStore}
	h.CategorySvc = &categoryService{store: catStore}
	h.Manager = &loginManagerAdapter{mgr: mgr}
}

// --- Account handlers ---

func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	if h.AccountSvc == nil {
		jsonOK(w, []interface{}{})
		return
	}
	filter := AccountFilter{
		Status:   r.URL.Query().Get("status"),
		Search:   r.URL.Query().Get("search"),
		Page:     getQueryInt(r, "page", 1),
		PageSize: getQueryInt(r, "page_size", 20),
	}
	if v := r.URL.Query().Get("category_id"); v != "" {
		var id int64
		fmt.Sscanf(v, "%d", &id)
		filter.CategoryID = id
	}
	accounts, total, err := h.AccountSvc.List(filter)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]interface{}{
		"items": accounts,
		"total": total,
		"page":  filter.Page,
	})
}

func (h *Handler) addAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Proxy string `json:"proxy"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Phone == "" {
		jsonError(w, "phone required", 400)
		return
	}
	if h.Manager != nil {
		id, err := h.Manager.AddAccount(req.Phone, req.Proxy)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		a, _ := h.AccountSvc.GetByID(id)
		h.WS.Broadcast("account_created", a)
		jsonOK(w, a)
	} else {
		a, err := h.AccountSvc.Create(req.Phone, req.Proxy)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		h.WS.Broadcast("account_created", a)
		jsonOK(w, a)
	}
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if h.AccountSvc == nil {
		jsonError(w, "not available", 503)
		return
	}
	a, err := h.AccountSvc.GetByID(id)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}
	jsonOK(w, a)
}

func (h *Handler) updateAccount(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	var req map[string]interface{}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	// Only allow safe fields
	fields := map[string]interface{}{}
	for _, k := range []string{"proxy", "remark", "twofa_password", "recovery_email", "is_active"} {
		if v, ok := req[k]; ok {
			fields[k] = v
		}
	}
	if v, ok := req["category_id"]; ok {
		switch n := v.(type) {
		case float64:
			if n > 0 {
				fields["category_id"] = int64(n)
			} else {
				fields["category_id"] = nil
			}
		}
	}
	if err := h.AccountSvc.Update(id, fields); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	a, _ := h.AccountSvc.GetByID(id)
	h.WS.Broadcast("account_updated", a)
	jsonOK(w, a)
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if h.Manager != nil {
		h.Manager.(*loginManagerAdapter).mgr.LogoutAccount(id)
	}
	if err := h.AccountSvc.Delete(id); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	h.WS.Broadcast("account_deleted", map[string]int64{"id": id})
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *Handler) startLogin(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if h.Manager == nil {
		jsonError(w, "manager not available", 503)
		return
	}
	a, err := h.AccountSvc.GetByID(id)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}
	p, err := h.Manager.RequestLogin(a.Phone)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	// Background: forward errors/done to WS
	go func() {
		select {
		case err := <-p.ErrChan:
			h.WS.Broadcast("login_error", map[string]interface{}{"phone": a.Phone, "error": err.Error()})
		case <-p.DoneChan:
			h.WS.Broadcast("login_done", map[string]interface{}{"phone": a.Phone})
		}
	}()

	jsonOK(w, map[string]string{"status": "code_sent", "phone": a.Phone})
}

func (h *Handler) submitCode(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	a, err := h.AccountSvc.GetByID(id)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if err := h.Manager.SubmitCode(a.Phone, req.Code); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonOK(w, map[string]string{"status": "code_submitted"})
}

func (h *Handler) submitPassword(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	a, err := h.AccountSvc.GetByID(id)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if err := h.Manager.SubmitPassword(a.Phone, req.Password); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonOK(w, map[string]string{"status": "password_submitted"})
}

func (h *Handler) logoutAccount(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if h.Manager != nil {
		h.Manager.(*loginManagerAdapter).mgr.LogoutAccount(id)
	}
	h.AccountSvc.Update(id, map[string]interface{}{"status": "offline"})
	h.WS.Broadcast("account_logout", map[string]int64{"id": id})
	jsonOK(w, map[string]string{"status": "logged_out"})
}
