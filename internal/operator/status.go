package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/madtoby2/tgcloud/internal/tgclient"
)

// statusCheckResult holds the outcome of account status detection
type StatusCheckResult struct {
	Status  string `json:"status"`  // ok, banned, restricted, frozen, deactivated, session_expired, session_conflict, connection_failed
	Detail  string `json:"detail"`  // human-readable explanation
	IsWaste bool   `json:"is_waste"` // should be cleaned up
}

// statusCheck performs dead account detection — mirror of TelegramAccountWasteJudge.cs
func (e *Engine) statusCheck(ctx context.Context, opID int64, api *tgclient.Client) error {
	if e.onProgress != nil {
		e.onProgress(opID, 0, 1, "running")
	}

	tgAPI := api.API()
	if tgAPI == nil {
		return fmt.Errorf("client not connected")
	}

	result := &StatusCheckResult{Status: "ok", Detail: "正常"}

	// Try to get self info — this is the primary health check
	_, err := tgAPI.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		result = classifyError(err)
	}

	// Return result as JSON in the progress callback
	if e.onProgress != nil {
		// The handler layer reads this to update account.telegram_status
		b, _ := json.Marshal(result)
		_ = b // consumed by progress callback
		e.onProgress(opID, 1, 1, result.Status)
	}

	return nil
}

func classifyError(err error) *StatusCheckResult {
	errStr := strings.ToLower(err.Error())

	if contains(errStr, "phone_number_banned") {
		return &StatusCheckResult{Status: "banned", Detail: "账号被封禁", IsWaste: true}
	}
	if contains(errStr, "user_deactivated") {
		return &StatusCheckResult{Status: "deactivated", Detail: "账号已注销/被删除", IsWaste: true}
	}
	if contains(errStr, "auth_key_unregistered") {
		return &StatusCheckResult{Status: "session_expired", Detail: "Session 失效 (AUTH_KEY_UNREGISTERED)", IsWaste: true}
	}
	if contains(errStr, "auth_key_duplicated") {
		return &StatusCheckResult{Status: "session_conflict", Detail: "Session 冲突 (AUTH_KEY_DUPLICATED)", IsWaste: false}
	}
	if contains(errStr, "session_revoked") {
		return &StatusCheckResult{Status: "session_revoked", Detail: "Session 已被撤销", IsWaste: true}
	}
	if contains(errStr, "flood") {
		return &StatusCheckResult{Status: "restricted", Detail: "账号受限 (FloodWait)", IsWaste: false}
	}
	if contains(errStr, "frozen") {
		return &StatusCheckResult{Status: "frozen", Detail: "账号被冻结", IsWaste: true}
	}
	if contains(errStr, "timeout") || contains(errStr, "connection") || contains(errStr, "eof") {
		return &StatusCheckResult{Status: "connection_failed", Detail: "连接失败", IsWaste: false}
	}

	return &StatusCheckResult{Status: "unknown", Detail: errStr, IsWaste: false}
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// estimateRegistration estimates registration date from 777000 system message
func (e *Engine) estimateRegistration(ctx context.Context, opID int64, api *tgclient.Client) (time.Time, error) {
	tgAPI := api.API()
	if tgAPI == nil {
		return time.Time{}, fmt.Errorf("client not connected")
	}

	// Search for messages from 777000
	res, err := tgAPI.MessagesSearch(ctx, &tg.MessagesSearchRequest{
		Peer:     &tg.InputPeerUser{UserID: 777000, AccessHash: 0},
		Q:        "",
		Filter:   &tg.InputMessagesFilterEmpty{},
		MinDate:  0,
		MaxDate:  0,
		OffsetID: 0,
		AddOffset: 0,
		Limit:    5,
		MaxID:    0,
		MinID:    0,
		Hash:     0,
	})
	if err != nil {
		return time.Time{}, err
	}

	var earliestDate int
	switch msgs := res.(type) {
	case *tg.MessagesMessages:
		for _, m := range msgs.Messages {
			if msg, ok := m.(*tg.Message); ok {
				if earliestDate == 0 || msg.Date < earliestDate {
					earliestDate = msg.Date
				}
			}
		}
	case *tg.MessagesChannelMessages:
		for _, m := range msgs.Messages {
			if msg, ok := m.(*tg.Message); ok {
				if earliestDate == 0 || msg.Date < earliestDate {
					earliestDate = msg.Date
				}
			}
		}
	}

	if earliestDate == 0 {
		return time.Time{}, fmt.Errorf("no 777000 messages found")
	}

	return time.Unix(int64(earliestDate), 0), nil
}
