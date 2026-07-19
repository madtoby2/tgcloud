package store

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// Category DB row
type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
}

// CategoryStore handles category persistence
type CategoryStore struct {
	db *sql.DB
}

func NewCategoryStore(db *sql.DB) *CategoryStore {
	return &CategoryStore{db: db}
}

func (s *CategoryStore) Create(name, color string) (*Category, error) {
	if color == "" {
		color = "#6366f1"
	}
	r, err := s.db.Exec("INSERT INTO categories (name, color) VALUES (?, ?)", name, color)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return s.GetByID(id)
}

func (s *CategoryStore) GetByID(id int64) (*Category, error) {
	c := &Category{}
	err := s.db.QueryRow(
		"SELECT id, name, color, sort_order, created_at FROM categories WHERE id=?", id,
	).Scan(&c.ID, &c.Name, &c.Color, &c.SortOrder, &c.CreatedAt)
	return c, err
}

func (s *CategoryStore) List() ([]*Category, error) {
	rows, err := s.db.Query("SELECT id, name, color, sort_order, created_at FROM categories ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []*Category
	for rows.Next() {
		c := &Category{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}

func (s *CategoryStore) Update(id int64, name, color string) error {
	_, err := s.db.Exec("UPDATE categories SET name=?, color=? WHERE id=?", name, color, id)
	return err
}

func (s *CategoryStore) Delete(id int64) error {
	// Unlink accounts first
	s.db.Exec("UPDATE accounts SET category_id=NULL WHERE category_id=?", id)
	_, err := s.db.Exec("DELETE FROM categories WHERE id=?", id)
	return err
}

// Session operations
func (s *CategoryStore) SaveSession(accountID int64, data []byte, db *sql.DB) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO sessions (account_id, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)",
		accountID, data,
	)
	return err
}

func (s *CategoryStore) GetSession(accountID int64, db *sql.DB) ([]byte, error) {
	var data []byte
	err := db.QueryRow("SELECT data FROM sessions WHERE account_id=?", accountID).Scan(&data)
	return data, err
}

func (s *CategoryStore) DeleteSession(accountID int64, db *sql.DB) error {
	_, err := db.Exec("DELETE FROM sessions WHERE account_id=?", accountID)
	return err
}

// Operation helpers
type Operation struct {
	ID            int64           `json:"id"`
	AccountID     int64           `json:"account_id"`
	BatchTaskID   *int64          `json:"batch_task_id"`
	Type          string          `json:"type"`
	Status        string          `json:"status"`
	ProgressTotal int             `json:"progress_total"`
	ProgressDone  int             `json:"progress_done"`
	Params        json.RawMessage `json:"params"`
	Result        json.RawMessage `json:"result"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

type BatchTask struct {
	ID         int64            `json:"id"`
	TaskType   string           `json:"task_type"`
	Status     string           `json:"status"`
	Total      int              `json:"total"`
	Completed  int              `json:"completed"`
	Failed     int              `json:"failed"`
	AccountIDs []int64          `json:"account_ids"`
	Config     json.RawMessage  `json:"config"`
	Result     json.RawMessage  `json:"result"`
	StartedAt  *string          `json:"started_at"`
	CompletedAt *string         `json:"completed_at"`
	CreatedAt  string           `json:"created_at"`
}

type ScheduledTask struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	TaskType   string           `json:"task_type"`
	CronExpr   string           `json:"cron_expr"`
	Config     json.RawMessage  `json:"config"`
	AccountIDs []int64          `json:"account_ids"`
	IsEnabled  bool             `json:"is_enabled"`
	LastRunAt  *string          `json:"last_run_at"`
	NextRunAt  *string          `json:"next_run_at"`
	CreatedAt  string           `json:"created_at"`
}

func IDListToJSON(ids []int64) string {
	if ids == nil {
		ids = []int64{}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func IDListFromJSON(s string) []int64 {
	var ids []int64
	if s == "" {
		return ids
	}
	json.Unmarshal([]byte(s), &ids)
	if ids == nil {
		ids = []int64{}
	}
	return ids
}

func NullableString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func MapContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
