package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/madtoby2/tgcloud/internal/tgclient"
	"go.uber.org/zap"
)

// Engine executes operations on TG accounts
type Engine struct {
	logger  *zap.Logger
	mu      sync.RWMutex
	jobs    map[int64]context.CancelFunc // op id → cancel

	onProgress func(opID int64, done, total int, status string)
}

func New(logger *zap.Logger) *Engine {
	return &Engine{
		logger: logger,
		jobs:   make(map[int64]context.CancelFunc),
	}
}

func (e *Engine) SetProgressCallback(fn func(opID int64, done, total int, status string)) {
	e.onProgress = fn
}

// Execute runs an operation with params
func (e *Engine) Execute(opID int64, accountID int64, opType string, params json.RawMessage, api *tgclient.Client) {
	ctx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	e.jobs[opID] = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.jobs, opID)
			e.mu.Unlock()
		}()

		var err error
		switch opType {
		case "status_check":
			err = e.statusCheck(ctx, opID, api)
		case "send_message":
			err = e.sendMessage(ctx, opID, params, api)
		case "join_group":
			err = e.joinGroups(ctx, opID, params, api)
		case "invite_users":
			err = e.inviteUsers(ctx, opID, params, api)
		case "farming":
			err = e.farming(ctx, opID, params, api)
		case "scrape_members":
			err = e.scrapeMembers(ctx, opID, params, api)
		case "phone_filter":
			err = e.phoneFilter(ctx, opID, params, api)
		case "search_groups":
			err = e.searchGroups(ctx, opID, params, api)
		case "clone_channel":
			err = e.cloneChannel(ctx, opID, params, api)
		default:
			err = fmt.Errorf("unknown operation type: %s", opType)
		}

		status := "done"
		if err != nil {
			status = "failed"
			e.logger.Error("operation failed",
				zap.Int64("op_id", opID),
				zap.String("type", opType),
				zap.Error(err),
			)
		}

		if e.onProgress != nil {
			e.onProgress(opID, 0, 0, status)
		}
	}()
}

// Cancel stops a running operation
func (e *Engine) Cancel(opID int64) {
	e.mu.RLock()
	cancel, ok := e.jobs[opID]
	e.mu.RUnlock()
	if ok {
		cancel()
	}
}

// Active returns number of running operations
func (e *Engine) Active() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.jobs)
}

// ExecuteBatch runs the same op on multiple accounts sequentially
func (e *Engine) ExecuteBatch(opIDs []int64, accountIDs []int64, opType string, params json.RawMessage, getAPI func(int64) *tgclient.Client) {
	for i, accountID := range accountIDs {
		api := getAPI(accountID)
		if api == nil {
			if e.onProgress != nil && i < len(opIDs) {
				e.onProgress(opIDs[i], 0, 0, "failed")
			}
			continue
		}
		e.Execute(opIDs[i], accountID, opType, params, api)
	}
}
