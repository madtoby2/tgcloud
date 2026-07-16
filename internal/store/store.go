package store

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type Account struct {
	ID          int64            `json:"id"`
	Phone       string           `json:"phone"`
	FirstName   string           `json:"first_name"`
	Username    string           `json:"username"`
	Status      string           `json:"status"` // online, offline, connecting, flood_wait
	Proxy       string           `json:"proxy"`  // socks5://user:pass@host:port
	FloodWait   int64            `json:"flood_wait"`
	Extra       *json.RawMessage `json:"extra,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type Operation struct {
	ID        int64           `json:"id"`
	AccountID int64           `json:"account_id"`
	Type      string          `json:"type"`   // join_group, send_message, invite, farming
	Status    string          `json:"status"` // pending, running, done, failed
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	return s, s.migrate()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phone TEXT NOT NULL,
		first_name TEXT DEFAULT '',
		username TEXT DEFAULT '',
		status TEXT DEFAULT 'offline',
		proxy TEXT DEFAULT '',
		flood_wait INTEGER DEFAULT 0,
		extra TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS operations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		params TEXT DEFAULT '{}',
		result TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS sessions (
		phone TEXT PRIMARY KEY,
		data BLOB,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

// --- Account CRUD ---

func (s *Store) CreateAccount(phone, proxy string) (*Account, error) {
	r, err := s.db.Exec("INSERT INTO accounts (phone, proxy) VALUES (?, ?)", phone, proxy)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return s.GetAccount(id)
}

func (s *Store) GetAccount(id int64) (*Account, error) {
	a := &Account{}
	var extra string
	err := s.db.QueryRow(
		"SELECT id, phone, first_name, username, status, proxy, flood_wait, extra, created_at, updated_at FROM accounts WHERE id=?",
		id,
	).Scan(&a.ID, &a.Phone, &a.FirstName, &a.Username, &a.Status, &a.Proxy, &a.FloodWait, &extra, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(extra)
	a.Extra = &raw
	return a, nil
}

func (s *Store) ListAccounts() ([]*Account, error) {
	rows, err := s.db.Query(
		"SELECT id, phone, first_name, username, status, proxy, flood_wait, extra, created_at, updated_at FROM accounts ORDER BY id DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		var extra string
		if err := rows.Scan(&a.ID, &a.Phone, &a.FirstName, &a.Username, &a.Status, &a.Proxy, &a.FloodWait, &extra, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		raw := json.RawMessage(extra)
		a.Extra = &raw
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Store) UpdateAccountStatus(id int64, status string, floodWait int64) error {
	_, err := s.db.Exec(
		"UPDATE accounts SET status=?, flood_wait=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		status, floodWait, id,
	)
	return err
}

func (s *Store) UpdateAccountInfo(id int64, firstName, username string) error {
	_, err := s.db.Exec(
		"UPDATE accounts SET first_name=?, username=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		firstName, username, id,
	)
	return err
}

func (s *Store) DeleteAccount(id int64) error {
	_, err := s.db.Exec("DELETE FROM accounts WHERE id=?", id)
	return err
}

// --- Session ---

func (s *Store) SaveSession(phone string, data []byte) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO sessions (phone, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)",
		phone, data,
	)
	return err
}

func (s *Store) GetSession(phone string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRow("SELECT data FROM sessions WHERE phone=?", phone).Scan(&data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) DeleteSession(phone string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE phone=?", phone)
	return err
}

// --- Operations ---

func (s *Store) CreateOperation(accountID int64, opType string, params json.RawMessage) (*Operation, error) {
	r, err := s.db.Exec(
		"INSERT INTO operations (account_id, type, params) VALUES (?, ?, ?)",
		accountID, opType, string(params),
	)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return s.GetOperation(id)
}

func (s *Store) GetOperation(id int64) (*Operation, error) {
	o := &Operation{}
	var params, result string
	err := s.db.QueryRow(
		"SELECT id, account_id, type, status, params, result, created_at, updated_at FROM operations WHERE id=?",
		id,
	).Scan(&o.ID, &o.AccountID, &o.Type, &o.Status, &params, &result, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	o.Params = json.RawMessage(params)
	o.Result = json.RawMessage(result)
	return o, nil
}

func (s *Store) ListOperations(accountID int64) ([]*Operation, error) {
	var rows *sql.Rows
	var err error
	if accountID > 0 {
		rows, err = s.db.Query(
			"SELECT id, account_id, type, status, params, result, created_at, updated_at FROM operations WHERE account_id=? ORDER BY id DESC LIMIT 100",
			accountID,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT id, account_id, type, status, params, result, created_at, updated_at FROM operations ORDER BY id DESC LIMIT 100",
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ops []*Operation
	for rows.Next() {
		o := &Operation{}
		var params, result string
		if err := rows.Scan(&o.ID, &o.AccountID, &o.Type, &o.Status, &params, &result, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		o.Params = json.RawMessage(params)
		o.Result = json.RawMessage(result)
		ops = append(ops, o)
	}
	return ops, nil
}

func (s *Store) UpdateOperation(id int64, status string, result json.RawMessage) error {
	_, err := s.db.Exec(
		"UPDATE operations SET status=?, result=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		status, string(result), id,
	)
	return err
}
