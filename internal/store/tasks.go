package store

import (
	"database/sql"
	"encoding/json"
)

// BatchTaskStore handles batch task persistence
type BatchTaskStore struct {
	db *sql.DB
}

func NewBatchTaskStore(db *sql.DB) *BatchTaskStore {
	return &BatchTaskStore{db: db}
}

func (s *BatchTaskStore) Create(taskType string, accountIDs []int64, config json.RawMessage) (*BatchTask, error) {
	r, err := s.db.Exec(
		`INSERT INTO batch_tasks (task_type, status, total, account_ids, config)
		 VALUES (?, 'pending', ?, ?, ?)`,
		taskType, len(accountIDs), IDListToJSON(accountIDs), string(config),
	)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return s.GetByID(id)
}

func (s *BatchTaskStore) GetByID(id int64) (*BatchTask, error) {
	t := &BatchTask{}
	var startedAt, completedAt sql.NullString
	var idsStr, configStr, resultStr string
	err := s.db.QueryRow(
		`SELECT id, task_type, status, total, completed, failed, account_ids, config, result, started_at, completed_at, created_at
		 FROM batch_tasks WHERE id=?`, id,
	).Scan(&t.ID, &t.TaskType, &t.Status, &t.Total, &t.Completed, &t.Failed,
		&idsStr, &configStr, &resultStr,
		&startedAt, &completedAt, &t.CreatedAt)

	if err != nil {
		return nil, err
	}
	t.AccountIDs = IDListFromJSON(idsStr)
	t.Config = json.RawMessage(configStr)
	t.Result = json.RawMessage(resultStr)
	if startedAt.Valid {
		t.StartedAt = &startedAt.String
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.String
	}
	return t, nil
}

func (s *BatchTaskStore) List() ([]*BatchTask, error) {
	rows, err := s.db.Query(
		`SELECT id, task_type, status, total, completed, failed, account_ids, config, result, started_at, completed_at, created_at
		 FROM batch_tasks ORDER BY created_at DESC LIMIT 50`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*BatchTask
	for rows.Next() {
		t := &BatchTask{}
		var startedAt, completedAt sql.NullString
		var idsStr, configStr, resultStr string
		if err := rows.Scan(&t.ID, &t.TaskType, &t.Status, &t.Total, &t.Completed, &t.Failed,
			&idsStr, &configStr, &resultStr,
			&startedAt, &completedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.AccountIDs = IDListFromJSON(idsStr)
		t.Config = json.RawMessage(configStr)
		t.Result = json.RawMessage(resultStr)
		if startedAt.Valid {
			t.StartedAt = &startedAt.String
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.String
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *BatchTaskStore) UpdateStatus(id int64, status string) error {
	query := `UPDATE batch_tasks SET status=?`
	args := []interface{}{status}
	if status == "running" {
		query += `, started_at=CURRENT_TIMESTAMP`
	}
	if status == "completed" || status == "failed" {
		query += `, completed_at=CURRENT_TIMESTAMP`
	}
	query += ` WHERE id=?`
	args = append(args, id)
	_, err := s.db.Exec(query, args...)
	return err
}

func (s *BatchTaskStore) UpdateProgress(id int64, completed, failed int) error {
	_, err := s.db.Exec(
		`UPDATE batch_tasks SET completed=?, failed=? WHERE id=?`,
		completed, failed, id,
	)
	return err
}

func (s *BatchTaskStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM batch_tasks WHERE id=?", id)
	return err
}

// ScheduledTaskStore handles scheduled task persistence
type ScheduledTaskStore struct {
	db *sql.DB
}

func NewScheduledTaskStore(db *sql.DB) *ScheduledTaskStore {
	return &ScheduledTaskStore{db: db}
}

func (s *ScheduledTaskStore) Create(name, taskType, cronExpr string, accountIDs []int64, config json.RawMessage) (*ScheduledTask, error) {
	r, err := s.db.Exec(
		`INSERT INTO scheduled_tasks (name, task_type, cron_expr, config, account_ids, is_enabled)
		 VALUES (?, ?, ?, ?, ?, 1)`,
		name, taskType, cronExpr, string(config), IDListToJSON(accountIDs),
	)
	if err != nil {
		return nil, err
	}
	id, _ := r.LastInsertId()
	return s.GetByID(id)
}

func (s *ScheduledTaskStore) GetByID(id int64) (*ScheduledTask, error) {
	t := &ScheduledTask{}
	var lastRun, nextRun sql.NullString
	var configStr, idsStr string
	err := s.db.QueryRow(
		`SELECT id, name, task_type, cron_expr, config, account_ids, is_enabled, last_run_at, next_run_at, created_at
		 FROM scheduled_tasks WHERE id=?`, id,
	).Scan(&t.ID, &t.Name, &t.TaskType, &t.CronExpr, &configStr, &idsStr,
		&t.IsEnabled, &lastRun, &nextRun, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.Config = json.RawMessage(configStr)
	t.AccountIDs = IDListFromJSON(idsStr)
	if lastRun.Valid {
		t.LastRunAt = &lastRun.String
	}
	if nextRun.Valid {
		t.NextRunAt = &nextRun.String
	}
	return t, nil
}

func (s *ScheduledTaskStore) List() ([]*ScheduledTask, error) {
	rows, err := s.db.Query(
		`SELECT id, name, task_type, cron_expr, config, account_ids, is_enabled, last_run_at, next_run_at, created_at
		 FROM scheduled_tasks ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*ScheduledTask
	for rows.Next() {
		t := &ScheduledTask{}
		var lastRun, nextRun sql.NullString
		var configStr, idsStr string
		if err := rows.Scan(&t.ID, &t.Name, &t.TaskType, &t.CronExpr, &configStr, &idsStr,
			&t.IsEnabled, &lastRun, &nextRun, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Config = json.RawMessage(configStr)
		t.AccountIDs = IDListFromJSON(idsStr)
		if lastRun.Valid {
			t.LastRunAt = &lastRun.String
		}
		if nextRun.Valid {
			t.NextRunAt = &nextRun.String
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *ScheduledTaskStore) Update(id int64, name, taskType, cronExpr string, accountIDs []int64, config json.RawMessage) error {
	_, err := s.db.Exec(
		`UPDATE scheduled_tasks SET name=?, task_type=?, cron_expr=?, config=?, account_ids=? WHERE id=?`,
		name, taskType, cronExpr, string(config), IDListToJSON(accountIDs), id,
	)
	return err
}

func (s *ScheduledTaskStore) Toggle(id int64) (bool, error) {
	var current bool
	s.db.QueryRow("SELECT is_enabled FROM scheduled_tasks WHERE id=?", id).Scan(&current)
	newVal := !current
	_, err := s.db.Exec("UPDATE scheduled_tasks SET is_enabled=? WHERE id=?", newVal, id)
	return newVal, err
}

func (s *ScheduledTaskStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM scheduled_tasks WHERE id=?", id)
	return err
}
