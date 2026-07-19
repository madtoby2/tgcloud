package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	return s, s.Migrate()
}

func (s *Store) DB() *sql.DB { return s.db }
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		color TEXT DEFAULT '#6366f1',
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phone TEXT NOT NULL,
		user_id INTEGER DEFAULT 0,
		first_name TEXT DEFAULT '',
		last_name TEXT DEFAULT '',
		username TEXT DEFAULT '',
		status TEXT DEFAULT 'offline',
		telegram_status TEXT DEFAULT '',
		telegram_status_detail TEXT DEFAULT '',
		telegram_status_checked_at DATETIME,
		category_id INTEGER REFERENCES categories(id),
		proxy TEXT DEFAULT '',
		twofa_password TEXT DEFAULT '',
		recovery_email TEXT DEFAULT '',
		estimated_registration_at DATETIME,
		last_login_at DATETIME,
		last_sync_at DATETIME,
		session_path TEXT DEFAULT '',
		is_active INTEGER DEFAULT 1,
		remark TEXT DEFAULT '',
		extra TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id INTEGER NOT NULL,
		access_hash INTEGER DEFAULT NULL,
		title TEXT NOT NULL,
		username TEXT DEFAULT '',
		is_broadcast INTEGER DEFAULT 0,
		member_count INTEGER DEFAULT 0,
		about TEXT DEFAULT '',
		creator_account_id INTEGER REFERENCES accounts(id),
		category TEXT DEFAULT '',
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS groups_chat (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id INTEGER NOT NULL,
		access_hash INTEGER DEFAULT NULL,
		title TEXT NOT NULL,
		username TEXT DEFAULT '',
		member_count INTEGER DEFAULT 0,
		about TEXT DEFAULT '',
		creator_account_id INTEGER REFERENCES accounts(id),
		category TEXT DEFAULT '',
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS account_channels (
		account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
		channel_id INTEGER REFERENCES channels(id) ON DELETE CASCADE,
		role TEXT DEFAULT 'member',
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (account_id, channel_id)
	);

	CREATE TABLE IF NOT EXISTS account_groups (
		account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
		group_id INTEGER REFERENCES groups_chat(id) ON DELETE CASCADE,
		role TEXT DEFAULT 'member',
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (account_id, group_id)
	);

	CREATE TABLE IF NOT EXISTS batch_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_type TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		total INTEGER DEFAULT 0,
		completed INTEGER DEFAULT 0,
		failed INTEGER DEFAULT 0,
		account_ids TEXT DEFAULT '[]',
		config TEXT DEFAULT '{}',
		result TEXT DEFAULT '{}',
		started_at DATETIME,
		completed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS scheduled_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		task_type TEXT NOT NULL,
		cron_expr TEXT NOT NULL,
		config TEXT DEFAULT '{}',
		account_ids TEXT DEFAULT '[]',
		is_enabled INTEGER DEFAULT 1,
		last_run_at DATETIME,
		next_run_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS operations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		batch_task_id INTEGER REFERENCES batch_tasks(id),
		type TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		progress_total INTEGER DEFAULT 0,
		progress_done INTEGER DEFAULT 0,
		params TEXT DEFAULT '{}',
		result TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
		data BLOB,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`)
	return err
}

// Helper: null time scan
func scanNullTime(val interface{}) *time.Time {
	switch v := val.(type) {
	case time.Time:
		return &v
	case *time.Time:
		return v
	default:
		return nil
	}
}

// Helper: null string scan
func nullString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// Helper: null int64 scan
func nullInt64(val interface{}) int64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}
