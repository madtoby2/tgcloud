package handler

import (
	"encoding/json"
	"net/http"

	"github.com/madtoby2/tgcloud/internal/store"
)

// --- Operations ---

func (h *Handler) listOperations(w http.ResponseWriter, r *http.Request) {
	accountID := getInt64Param(r, "account_id")
	db := h.store.DB()
	query := "SELECT id, account_id, type, status, progress_total, progress_done, params, result, created_at, updated_at FROM operations WHERE 1=1"
	args := []interface{}{}
	if accountID > 0 {
		query += " AND account_id = ?"
		args = append(args, accountID)
	}
	query += " ORDER BY id DESC LIMIT 100"
	rows, err := db.Query(query, args...)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var ops []OperationRow
	for rows.Next() {
		var o OperationRow
		var btID *int64
		var params, result string
		if err := rows.Scan(&o.ID, &o.AccountID, &o.Type, &o.Status,
			&o.ProgressTotal, &o.ProgressDone, &params, &result, &o.CreatedAt, &o.CreatedAt); err != nil {
			continue
		}
		_ = btID
		o.Params = json.RawMessage(params)
		o.Result = json.RawMessage(result)
		ops = append(ops, o)
	}
	jsonOK(w, ops)
}

func (h *Handler) createOperation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID int64           `json:"account_id"`
		Type      string          `json:"type"`
		Params    json.RawMessage `json:"params"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if req.AccountID == 0 || req.Type == "" {
		jsonError(w, "account_id and type required", 400)
		return
	}

	// Create operation record
	db := h.store.DB()
	r2, err := db.Exec(
		`INSERT INTO operations (account_id, type, status, params) VALUES (?, ?, 'pending', ?)`,
		req.AccountID, req.Type, string(req.Params),
	)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	opID, _ := r2.LastInsertId()

	// Trigger execution
	if h.engine != nil && h.mgr != nil {
		api := h.mgr.GetClient(req.AccountID)
		if api != nil {
			// Update status to running
			db.Exec("UPDATE operations SET status='running' WHERE id=?", opID)
			h.engine.Execute(opID, req.AccountID, req.Type, req.Params, api)
		}
	}

	jsonOK(w, map[string]interface{}{
		"id":         opID,
		"account_id": req.AccountID,
		"type":       req.Type,
		"status":     "pending",
	})
}

func (h *Handler) cancelOperation(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if h.engine != nil {
		h.engine.Cancel(id)
	}
	h.store.DB().Exec("UPDATE operations SET status='cancelled' WHERE id=?", id)
	jsonOK(w, map[string]string{"status": "cancelled"})
}

// --- Batch Tasks ---

func (h *Handler) listBatchTasks(w http.ResponseWriter, r *http.Request) {
	btStore := store.NewBatchTaskStore(h.store.DB())
	tasks, err := btStore.List()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if tasks == nil {
		tasks = []*store.BatchTask{}
	}
	jsonOK(w, tasks)
}

func (h *Handler) createBatchTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskType   string          `json:"task_type"`
		AccountIDs []int64         `json:"account_ids"`
		Config     json.RawMessage `json:"config"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	if req.TaskType == "" || len(req.AccountIDs) == 0 {
		jsonError(w, "task_type and account_ids required", 400)
		return
	}

	btStore := store.NewBatchTaskStore(h.store.DB())
	task, err := btStore.Create(req.TaskType, req.AccountIDs, req.Config)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	h.WS.Broadcast("batch_task_created", task)
	jsonOK(w, task)
}

func (h *Handler) startBatchTask(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	btStore := store.NewBatchTaskStore(h.store.DB())
	task, err := btStore.GetByID(id)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}

	if h.engine == nil || h.mgr == nil {
		jsonError(w, "engine not available", 503)
		return
	}

	btStore.UpdateStatus(id, "running")

	// Create operations for each account
	go func() {
		completed, failed := 0, 0
		for _, accountID := range task.AccountIDs {
			r, err := h.store.DB().Exec(
				`INSERT INTO operations (account_id, batch_task_id, type, status, params)
				 VALUES (?, ?, ?, 'running', ?)`,
				accountID, id, task.TaskType, string(task.Config),
			)
			if err != nil {
				failed++
				continue
			}
			opID, _ := r.LastInsertId()

			api := h.mgr.GetClient(accountID)
			if api == nil {
				h.store.DB().Exec("UPDATE operations SET status='failed', result='{\"error\":\"not connected\"}' WHERE id=?", opID)
				failed++
			} else {
				h.engine.Execute(opID, accountID, task.TaskType, task.Config, api)
				completed++
			}

			btStore.UpdateProgress(id, completed, failed)
		}

		if failed == len(task.AccountIDs) {
			btStore.UpdateStatus(id, "failed")
		} else {
			btStore.UpdateStatus(id, "completed")
		}
		h.WS.Broadcast("batch_task_updated", nil)
	}()

	jsonOK(w, map[string]string{"status": "started"})
}

func (h *Handler) pauseBatchTask(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	store.NewBatchTaskStore(h.store.DB()).UpdateStatus(id, "paused")
	jsonOK(w, map[string]string{"status": "paused"})
}

func (h *Handler) cancelBatchTask(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	store.NewBatchTaskStore(h.store.DB()).UpdateStatus(id, "canceled")
	jsonOK(w, map[string]string{"status": "canceled"})
}

// --- Scheduled Tasks ---

func (h *Handler) listScheduledTasks(w http.ResponseWriter, r *http.Request) {
	schStore := store.NewScheduledTaskStore(h.store.DB())
	tasks, err := schStore.List()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if tasks == nil {
		tasks = []*store.ScheduledTask{}
	}
	jsonOK(w, tasks)
}

func (h *Handler) createScheduledTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string          `json:"name"`
		TaskType   string          `json:"task_type"`
		CronExpr   string          `json:"cron_expr"`
		AccountIDs []int64         `json:"account_ids"`
		Config     json.RawMessage `json:"config"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	schStore := store.NewScheduledTaskStore(h.store.DB())
	task, err := schStore.Create(req.Name, req.TaskType, req.CronExpr, req.AccountIDs, req.Config)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, task)
}

func (h *Handler) updateScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	var req struct {
		Name       string          `json:"name"`
		TaskType   string          `json:"task_type"`
		CronExpr   string          `json:"cron_expr"`
		AccountIDs []int64         `json:"account_ids"`
		Config     json.RawMessage `json:"config"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}
	schStore := store.NewScheduledTaskStore(h.store.DB())
	if err := schStore.Update(id, req.Name, req.TaskType, req.CronExpr, req.AccountIDs, req.Config); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]string{"status": "updated"})
}

func (h *Handler) deleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	if err := store.NewScheduledTaskStore(h.store.DB()).Delete(id); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (h *Handler) toggleScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := getInt64Param(r, "id")
	enabled, err := store.NewScheduledTaskStore(h.store.DB()).Toggle(id)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]bool{"enabled": enabled})
}
