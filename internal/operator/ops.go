package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gotd/td/tg"
	"github.com/madtoby2/tgcloud/internal/tgclient"
)

// sendMessage sends text to multiple targets
func (e *Engine) sendMessage(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	var p struct {
		Targets  []string `json:"targets"`
		Message  string   `json:"message"`
		Interval int      `json:"interval"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	if p.Interval <= 0 {
		p.Interval = 3
	}

	tgAPI := api.API()
	if tgAPI == nil {
		return fmt.Errorf("client not connected")
	}

	sent, failed := 0, 0
	for _, target := range p.Targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		peer, err := resolvePeer(ctx, tgAPI, target)
		if err != nil {
			failed++
			continue
		}

		_, err = tgAPI.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:     peer,
			Message:  p.Message,
			RandomID: tgclient.RandomInt63(),
		})
		if err != nil {
			if wait := tgclient.WaitFloodWait(err); wait > 0 {
				// Sleep then retry once
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait + time.Second):
				}
				_, err = tgAPI.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
					Peer: peer, Message: p.Message, RandomID: tgclient.RandomInt63(),
				})
			}
			if err != nil {
				failed++
				continue
			}
		}
		sent++

		if e.onProgress != nil {
			e.onProgress(opID, sent+failed, len(p.Targets), "running")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(p.Interval) * time.Second):
		}
	}

	return nil
}

// joinGroups joins groups via invite links
func (e *Engine) joinGroups(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	var p struct {
		Links    []string `json:"links"`
		Interval int      `json:"interval"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	if p.Interval <= 0 {
		p.Interval = 5
	}

	tgAPI := api.API()
	if tgAPI == nil {
		return fmt.Errorf("client not connected")
	}

	joined, failed := 0, 0
	for _, link := range p.Links {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Extract hash from invite link
		hash := extractInviteHash(link)
		if hash == "" {
			failed++
			continue
		}

		_, err := tgAPI.MessagesImportChatInvite(ctx, hash)
		if err != nil {
			failed++
			continue
		}
		joined++

		if e.onProgress != nil {
			e.onProgress(opID, joined+failed, len(p.Links), "running")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(p.Interval) * time.Second):
		}
	}
	return nil
}

// inviteUsers invites users to a channel
func (e *Engine) inviteUsers(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	// Stub: implement full invite logic
	return nil
}

// farming rotates messages in groups for warmup
func (e *Engine) farming(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	// Stub
	return nil
}

// scrapeMembers collects group members
func (e *Engine) scrapeMembers(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	// Stub
	return nil
}

// phoneFilter checks if phone numbers are registered on TG
func (e *Engine) phoneFilter(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	// Stub
	return nil
}

// searchGroups performs global search
func (e *Engine) searchGroups(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	// Stub
	return nil
}

// cloneChannel copies messages from source to target
func (e *Engine) cloneChannel(ctx context.Context, opID int64, params json.RawMessage, api *tgclient.Client) error {
	// Stub
	return nil
}
