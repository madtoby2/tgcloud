package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/madtoby2/tgcloud/internal/manager"
	"github.com/madtoby2/tgcloud/internal/operator"
	"github.com/madtoby2/tgcloud/internal/store"
)

type Handler struct {
	WS  *WSHub
	store *store.Store
	engine *operator.Engine
	mgr   *manager.Manager

	// Will be populated in subsequent phases
	AccountSvc     AccountService
	CategorySvc    CategoryService
	ChannelSvc     ChannelService
	GroupSvc       GroupService
	OperationSvc   OperationService
	BatchTaskSvc   BatchTaskService
	ScheduledTaskSvc ScheduledTaskService
	Manager        LoginManager
}

// Service interfaces
type AccountService interface {
	List(filter AccountFilter) ([]AccountRow, int, error)
	GetByID(id int64) (*AccountRow, error)
	Create(phone, proxy string) (*AccountRow, error)
	Update(id int64, fields map[string]interface{}) error
	Delete(id int64) error
	SetTelegramStatus(id int64, status, detail string) error
	GetByPhone(phone string) (*AccountRow, error)
}

type AccountFilter struct {
	CategoryID int64
	Status     string
	Search     string
	Page       int
	PageSize   int
}

type AccountRow struct {
	ID    int64  `json:"id"`
	Phone string `json:"phone"`
	// More fields populated by store
	Raw map[string]interface{} `json:"-"`
}

func (a *AccountRow) MarshalJSON() ([]byte, error) {
	m := a.Raw
	if m == nil {
		m = make(map[string]interface{})
	}
	m["id"] = a.ID
	m["phone"] = a.Phone
	return json.Marshal(m)
}

type CategoryService interface {
	List() ([]CategoryRow, error)
	GetByID(id int64) (*CategoryRow, error)
	Create(name, color string) (*CategoryRow, error)
	Update(id int64, name, color string) error
	Delete(id int64) error
}

type CategoryRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

type ChannelService interface {
	List(filter EntityFilter) ([]EntityRow, int, error)
}

type GroupService interface {
	List(filter EntityFilter) ([]EntityRow, int, error)
}

type EntityFilter struct {
	AccountID int64
	Category  string
	Search    string
	Page      int
	PageSize  int
}

type EntityRow struct {
	ID         int64  `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	Title      string `json:"title"`
	Username   string `json:"username"`
	MemberCount int   `json:"member_count"`
}

type OperationService interface {
	List(accountID int64) ([]OperationRow, error)
	Create(accountID int64, opType string, params json.RawMessage) (*OperationRow, error)
	Cancel(id int64) error
}

type OperationRow struct {
	ID            int64           `json:"id"`
	AccountID     int64           `json:"account_id"`
	Type          string          `json:"type"`
	Status        string          `json:"status"`
	ProgressTotal int            `json:"progress_total"`
	ProgressDone  int            `json:"progress_done"`
	Params        json.RawMessage `json:"params"`
	Result        json.RawMessage `json:"result"`
	CreatedAt     string          `json:"created_at"`
}

type BatchTaskService interface {
	List() ([]BatchTaskRow, error)
	Create(taskType string, accountIDs []int64, config json.RawMessage) (*BatchTaskRow, error)
	Start(id int64) error
	Pause(id int64) error
	Cancel(id int64) error
}

type BatchTaskRow struct {
	ID         int64           `json:"id"`
	TaskType   string          `json:"task_type"`
	Status     string          `json:"status"`
	Total      int             `json:"total"`
	Completed  int             `json:"completed"`
	Failed     int             `json:"failed"`
	AccountIDs []int64         `json:"account_ids"`
	Config     json.RawMessage `json:"config"`
	Result     json.RawMessage `json:"result"`
}

type ScheduledTaskService interface {
	List() ([]ScheduledTaskRow, error)
	Create(name, taskType, cronExpr string, accountIDs []int64, config json.RawMessage) (*ScheduledTaskRow, error)
	Update(id int64, name, taskType, cronExpr string, accountIDs []int64, config json.RawMessage) error
	Delete(id int64) error
	Toggle(id int64) (bool, error)
}

type ScheduledTaskRow struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	TaskType   string          `json:"task_type"`
	CronExpr   string          `json:"cron_expr"`
	Config     json.RawMessage `json:"config"`
	AccountIDs []int64         `json:"account_ids"`
	IsEnabled  bool            `json:"is_enabled"`
	LastRunAt  string          `json:"last_run_at"`
	NextRunAt  string          `json:"next_run_at"`
}

type LoginManager interface {
	AddAccount(phone, proxy string) (int64, error)
	RequestLogin(phone string) (*PendingLogin, error)
	SubmitCode(phone, code string) error
	SubmitPassword(phone, password string) error
}

type PendingLogin struct {
	Phone    string
	CodeChan chan string
	PassChan chan string
	ErrChan  chan error
	DoneChan chan struct{}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		// Status
		r.Get("/status", h.systemStatus)
		r.Get("/dashboard", h.dashboard)

		// Accounts
		r.Get("/accounts", h.listAccounts)
		r.Post("/accounts", h.addAccount)
		r.Get("/accounts/{id}", h.getAccount)
		r.Put("/accounts/{id}", h.updateAccount)
		r.Delete("/accounts/{id}", h.deleteAccount)
		r.Post("/accounts/{id}/login", h.startLogin)
		r.Post("/accounts/{id}/code", h.submitCode)
		r.Post("/accounts/{id}/password", h.submitPassword)
		r.Post("/accounts/{id}/logout", h.logoutAccount)
		r.Post("/accounts/{id}/status-check", h.checkAccountStatus)
		r.Post("/accounts/{id}/estimate-registration", h.estimateRegistration)
		r.Post("/accounts/{id}/twofa/change", h.changeTwoFA)
		r.Post("/accounts/{id}/twofa/email", h.bindRecoveryEmail)
		r.Post("/accounts/batch/status-check", h.batchStatusCheck)
		r.Post("/accounts/batch/delete-waste", h.batchDeleteWaste)

		// Import/Export
		r.Post("/import/telethon", h.importTelethon)
		r.Post("/import/tdata", h.importTData)
		r.Get("/export/{id}/telethon", h.exportTelethon)

		// Categories
		r.Get("/categories", h.listCategories)
		r.Post("/categories", h.createCategory)
		r.Put("/categories/{id}", h.updateCategory)
		r.Delete("/categories/{id}", h.deleteCategory)

		// Channels/Groups
		r.Get("/channels", h.listChannels)
		r.Post("/channels/sync", h.syncChannels)
		r.Get("/groups", h.listGroups)
		r.Post("/groups/sync", h.syncGroups)

		// Operations
		r.Get("/operations", h.listOperations)
		r.Post("/operations", h.createOperation)
		r.Post("/operations/{id}/cancel", h.cancelOperation)

		// Batch Tasks
		r.Get("/batch-tasks", h.listBatchTasks)
		r.Post("/batch-tasks", h.createBatchTask)
		r.Post("/batch-tasks/{id}/start", h.startBatchTask)
		r.Post("/batch-tasks/{id}/pause", h.pauseBatchTask)
		r.Post("/batch-tasks/{id}/cancel", h.cancelBatchTask)

		// Scheduled Tasks
		r.Get("/scheduled-tasks", h.listScheduledTasks)
		r.Post("/scheduled-tasks", h.createScheduledTask)
		r.Put("/scheduled-tasks/{id}", h.updateScheduledTask)
		r.Delete("/scheduled-tasks/{id}", h.deleteScheduledTask)
		r.Post("/scheduled-tasks/{id}/toggle", h.toggleScheduledTask)
	})

	// WebSocket
	r.Get("/ws", h.WS.ServeWS)

	return r
}

// JSON helpers
func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "data": data})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": msg})
}
