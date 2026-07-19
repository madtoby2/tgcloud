package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Account DB row
type Account struct {
	ID                        int64   `json:"id"`
	Phone                     string  `json:"phone"`
	UserID                    int64   `json:"user_id"`
	FirstName                 string  `json:"first_name"`
	LastName                  string  `json:"last_name"`
	Username                  string  `json:"username"`
	Status                    string  `json:"status"`
	TelegramStatus            string  `json:"telegram_status"`
	TelegramStatusDetail      string  `json:"telegram_status_detail"`
	TelegramStatusCheckedAt   *string `json:"telegram_status_checked_at"`
	CategoryID                *int64  `json:"category_id"`
	Proxy                     string  `json:"proxy"`
	TwoFAPassword             string  `json:"twofa_password"`
	RecoveryEmail             string  `json:"recovery_email"`
	EstimatedRegistrationAt   *string `json:"estimated_registration_at"`
	LastLoginAt               *string `json:"last_login_at"`
	LastSyncAt                *string `json:"last_sync_at"`
	SessionPath               string  `json:"session_path"`
	IsActive                  bool    `json:"is_active"`
	Remark                    string  `json:"remark"`
	Extra                     string  `json:"extra"`
	CreatedAt                 string  `json:"created_at"`
	UpdatedAt                 string  `json:"updated_at"`
}

// AccountStore handles account persistence
type AccountStore struct {
	db *sql.DB
}

func NewAccountStore(db *sql.DB) *AccountStore {
	return &AccountStore{db: db}
}

func (s *AccountStore) DB() *sql.DB { return s.db }

func (s *AccountStore) Create(phone, proxy string) (*Account, error) {
	r, err := s.db.Exec(
		`INSERT INTO accounts (phone, proxy, status) VALUES (?, ?, 'offline')`,
		phone, proxy,
	)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return s.GetByID(id)
}

func (s *AccountStore) GetByID(id int64) (*Account, error) {
	a := &Account{}
	var catID sql.NullInt64
	var tgStatusChecked, estReg, lastLogin, lastSync sql.NullString

	err := s.db.QueryRow(
		`SELECT id, phone, user_id, first_name, last_name, username, status,
		 telegram_status, telegram_status_detail, telegram_status_checked_at,
		 category_id, proxy, twofa_password, recovery_email,
		 estimated_registration_at, last_login_at, last_sync_at,
		 session_path, is_active, remark, extra, created_at, updated_at
		 FROM accounts WHERE id=?`, id,
	).Scan(
		&a.ID, &a.Phone, &a.UserID, &a.FirstName, &a.LastName, &a.Username, &a.Status,
		&a.TelegramStatus, &a.TelegramStatusDetail, &tgStatusChecked,
		&catID, &a.Proxy, &a.TwoFAPassword, &a.RecoveryEmail,
		&estReg, &lastLogin, &lastSync,
		&a.SessionPath, &a.IsActive, &a.Remark, &a.Extra, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if tgStatusChecked.Valid {
		a.TelegramStatusCheckedAt = &tgStatusChecked.String
	}
	if catID.Valid {
		a.CategoryID = &catID.Int64
	}
	if estReg.Valid {
		a.EstimatedRegistrationAt = &estReg.String
	}
	if lastLogin.Valid {
		a.LastLoginAt = &lastLogin.String
	}
	if lastSync.Valid {
		a.LastSyncAt = &lastSync.String
	}

	return a, nil
}

func (s *AccountStore) GetByPhone(phone string) (*Account, error) {
	var id int64
	err := s.db.QueryRow("SELECT id FROM accounts WHERE phone=? LIMIT 1", phone).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

// AccountFilter for listing
type AccountFilter struct {
	CategoryID *int64
	Status     string
	Search     string
	OnlyWaste  bool
	Page       int
	PageSize   int
}

func (s *AccountStore) List(f AccountFilter) ([]*Account, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if f.CategoryID != nil {
		where = append(where, "category_id = ?")
		args = append(args, *f.CategoryID)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Search != "" {
		where = append(where, "(phone LIKE ? OR first_name LIKE ? OR username LIKE ? OR remark LIKE ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s, s, s)
	}
	if f.OnlyWaste {
		where = append(where, "telegram_status IN ('banned','deactivated','session_expired','session_revoked','connection_failed')")
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM accounts WHERE " + strings.Join(where, " AND ")
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Pagination
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	offset := (f.Page - 1) * f.PageSize

	query := `SELECT id, phone, user_id, first_name, last_name, username, status,
	 telegram_status, telegram_status_detail, telegram_status_checked_at,
	 category_id, proxy, twofa_password, recovery_email,
	 estimated_registration_at, last_login_at, last_sync_at,
	 session_path, is_active, remark, extra, created_at, updated_at
	 FROM accounts WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY updated_at DESC LIMIT ? OFFSET ?`

	args = append(args, f.PageSize, offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		var catID sql.NullInt64
		var tgStatusChecked, estReg, lastLogin, lastSync sql.NullString
		err := rows.Scan(
			&a.ID, &a.Phone, &a.UserID, &a.FirstName, &a.LastName, &a.Username, &a.Status,
			&a.TelegramStatus, &a.TelegramStatusDetail, &tgStatusChecked,
			&catID, &a.Proxy, &a.TwoFAPassword, &a.RecoveryEmail,
			&estReg, &lastLogin, &lastSync,
			&a.SessionPath, &a.IsActive, &a.Remark, &a.Extra, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if tgStatusChecked.Valid {
			a.TelegramStatusCheckedAt = &tgStatusChecked.String
		}
		if catID.Valid {
			a.CategoryID = &catID.Int64
		}
		if estReg.Valid {
			a.EstimatedRegistrationAt = &estReg.String
		}
		if lastLogin.Valid {
			a.LastLoginAt = &lastLogin.String
		}
		if lastSync.Valid {
			a.LastSyncAt = &lastSync.String
		}
		accounts = append(accounts, a)
	}
	return accounts, total, nil
}

func (s *AccountStore) Update(id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []interface{}{}
	for k, v := range fields {
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	args = append(args, id)
	query := "UPDATE accounts SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	_, err := s.db.Exec(query, args...)
	return err
}

func (s *AccountStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM accounts WHERE id=?", id)
	return err
}

func (s *AccountStore) SetTelegramStatus(id int64, status, detail string) error {
	_, err := s.db.Exec(
		`UPDATE accounts SET telegram_status=?, telegram_status_detail=?, telegram_status_checked_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, detail, id,
	)
	return err
}

func (s *AccountStore) DashboardStats() (map[string]int, error) {
	stats := map[string]int{"total": 0, "online": 0, "banned": 0, "restricted": 0, "frozen": 0, "deactivated": 0}

	var total, online, banned, restricted, frozen, deactivated int
	s.db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&total)
	s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE status='online'").Scan(&online)
	s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE telegram_status='banned'").Scan(&banned)
	s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE telegram_status='restricted'").Scan(&restricted)
	s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE telegram_status='frozen'").Scan(&frozen)
	s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE telegram_status='deactivated'").Scan(&deactivated)

	stats["total"] = total
	stats["online"] = online
	stats["banned"] = banned
	stats["restricted"] = restricted
	stats["frozen"] = frozen
	stats["deactivated"] = deactivated
	stats["normal"] = total - banned - restricted - frozen - deactivated

	return stats, nil
}

// ToJSON converts Account to a JSON-friendly map
func (a *Account) ToJSON() map[string]interface{} {
	extra := map[string]interface{}{}
	json.Unmarshal([]byte(a.Extra), &extra)

	m := map[string]interface{}{
		"id":                a.ID,
		"phone":             a.Phone,
		"user_id":           a.UserID,
		"first_name":        a.FirstName,
		"last_name":         a.LastName,
		"username":          a.Username,
		"status":            a.Status,
		"telegram_status":   a.TelegramStatus,
		"telegram_status_detail": a.TelegramStatusDetail,
		"proxy":             a.Proxy,
		"recovery_email":    a.RecoveryEmail,
		"session_path":      a.SessionPath,
		"is_active":         a.IsActive,
		"remark":            a.Remark,
		"extra":             extra,
		"created_at":        a.CreatedAt,
		"updated_at":        a.UpdatedAt,
	}
	if a.TelegramStatusCheckedAt != nil {
		m["telegram_status_checked_at"] = *a.TelegramStatusCheckedAt
	}
	if a.CategoryID != nil {
		m["category_id"] = *a.CategoryID
	}
	if a.EstimatedRegistrationAt != nil {
		m["estimated_registration_at"] = *a.EstimatedRegistrationAt
	}
	if a.LastLoginAt != nil {
		m["last_login_at"] = *a.LastLoginAt
	}
	if a.LastSyncAt != nil {
		m["last_sync_at"] = *a.LastSyncAt
	}
	return m
}
