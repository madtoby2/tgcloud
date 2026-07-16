package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"github.com/madtoby2/tgcloud/internal/store"
	"github.com/madtoby2/tgcloud/internal/tgclient"
	"go.uber.org/zap"
)

// Op types
const (
	OpJoinGroup  = "join_group"
	OpSendMsg    = "send_message"
	OpInvite     = "invite_users"
	OpFarming    = "farming"
	OpScrape     = "scrape_members"
	OpPhoneCheck = "phone_filter"
	OpSearch     = "search_groups"
	OpClone      = "clone_channel"
	OpBoost      = "boost"
	OpRedPacket  = "redpacket"
)

type Engine struct {
	store  *store.Store
	logger *zap.Logger

	mu   sync.Mutex
	jobs map[int64]context.CancelFunc
}

func New(st *store.Store, logger *zap.Logger) *Engine {
	return &Engine{
		store:  st,
		logger: logger,
		jobs:   make(map[int64]context.CancelFunc),
	}
}

func (e *Engine) Execute(op *store.Operation, api *tg.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.jobs[op.ID] = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.jobs, op.ID)
			e.mu.Unlock()
		}()

		result, err := e.run(ctx, op, api)
		status := "done"
		out := json.RawMessage(`{"ok":true}`)
		if err != nil {
			status = "failed"
			out, _ = json.Marshal(map[string]string{"error": err.Error()})
		} else if result != nil {
			out, _ = json.Marshal(result)
		}
		e.store.UpdateOperation(op.ID, status, out)
	}()
}

func (e *Engine) Cancel(opID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cancel, ok := e.jobs[opID]; ok {
		cancel()
	}
}

func (e *Engine) run(ctx context.Context, op *store.Operation, api *tg.Client) (interface{}, error) {
	switch op.Type {
	case OpJoinGroup:
		return e.joinGroup(ctx, api, op.Params)
	case OpSendMsg:
		return e.sendMessage(ctx, api, op.Params)
	case OpInvite:
		return e.inviteUsers(ctx, api, op.Params)
	case OpFarming:
		return e.farming(ctx, api, op.Params)
	case OpScrape:
		return e.scrapeMembers(ctx, api, op.Params)
	case OpPhoneCheck:
		return e.phoneFilter(ctx, api, op.Params)
	case OpSearch:
		return e.searchGroups(ctx, api, op.Params)
	case OpClone:
		return e.cloneChannel(ctx, api, op.Params)
	case OpBoost:
		return e.boost(ctx, api, op.Params)
	case OpRedPacket:
		return e.redpacket(ctx, api, op.Params)
	default:
		return nil, fmt.Errorf("unknown op type: %s", op.Type)
	}
}

// --- Join Group ---

func (e *Engine) joinGroup(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Links []string `json:"links"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	results := make([]map[string]interface{}, 0)
	for _, link := range p.Links {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		r := map[string]interface{}{"link": link}

		if strings.HasPrefix(link, "https://t.me/+") || strings.HasPrefix(link, "t.me/+") {
			hash := link
			hash = strings.TrimPrefix(hash, "https://t.me/")
			hash = strings.TrimPrefix(hash, "t.me/")
			hash = strings.TrimPrefix(hash, "+")

			_, err := api.MessagesImportChatInvite(ctx, hash)
			if err != nil {
				wait := tgclient.WaitFloodWait(err)
				if wait > 0 {
					r["flood_wait"] = wait.Seconds()
					time.Sleep(wait)
					continue
				}
				r["error"] = err.Error()
			} else {
				r["status"] = "joined"
			}
		} else {
			username := strings.TrimPrefix(link, "@")
			username = strings.TrimPrefix(username, "https://t.me/")
			username = strings.TrimPrefix(username, "t.me/")

			res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
				Username: username,
			})
			if err != nil {
				r["error"] = err.Error()
			} else {
				peer := toInputPeer(res.Peer)
				if peer == nil {
					r["error"] = "cannot resolve peer"
				} else if ch, ok := peer.(*tg.InputPeerChannel); ok {
					for _, chat := range res.Chats {
						if c, ok := chat.(*tg.Channel); ok && c.ID == ch.ChannelID {
							_, err := api.ChannelsJoinChannel(ctx, &tg.InputChannel{
								ChannelID:  c.ID,
								AccessHash: c.AccessHash,
							})
							if err != nil {
								r["error"] = err.Error()
							} else {
								r["channel_id"] = c.ID
								r["title"] = c.Title
								r["status"] = "joined"
							}
							break
						}
					}
				} else {
					r["error"] = "not a channel"
				}
			}
		}
		results = append(results, r)
		time.Sleep(time.Duration(2+rand.Intn(6)) * time.Second)
	}

	return map[string]interface{}{"results": results, "total": len(results)}, nil
}

// --- Send Message ---

func (e *Engine) sendMessage(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Targets  []string `json:"targets"`  // usernames or channel IDs
		Message  string   `json:"message"`
		Media    string   `json:"media"`   // path to media file (future)
		Interval int      `json:"interval"` // seconds between sends
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Interval <= 0 {
		p.Interval = 5
	}

	sent, failed := 0, 0

	// Resolve targets first
	type target struct {
		peer  tg.InputPeerClass
		label string
	}
	targets := make([]target, 0)

	for _, t := range p.Targets {
		peer, label, err := resolvePeer(ctx, api, t)
		if err != nil {
			failed++
			continue
		}
		targets = append(targets, target{peer, label})
	}

	// Send to each target
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:    t.peer,
			Message: p.Message,
			RandomID: rand.Int63(),
		})
		if err != nil {
			wait := tgclient.WaitFloodWait(err)
			if wait > 0 {
				e.logger.Warn("flood wait during send",
					zap.String("target", t.label),
					zap.Duration("wait", wait))
				time.Sleep(wait)
				continue
			}
			e.logger.Error("send failed", zap.String("target", t.label), zap.Error(err))
			failed++
		} else {
			sent++
		}

		// Random interval
		base := time.Duration(p.Interval) * time.Second
		jitter := time.Duration(rand.Intn(p.Interval)) * time.Second
		time.Sleep(base + jitter)
	}

	return map[string]interface{}{"sent": sent, "failed": failed}, nil
}

// --- Invite Users ---

func (e *Engine) inviteUsers(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Channel  string   `json:"channel"`  // target channel username/id
		Users    []string `json:"users"`    // usernames to invite
		MaxUsers int      `json:"max_users"` // per-account limit
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Resolve target channel
	channelPeer, _, err := resolvePeer(ctx, api, p.Channel)
	if err != nil {
		return nil, fmt.Errorf("resolve channel: %w", err)
	}
	chPeer, ok := channelPeer.(*tg.InputPeerChannel)
	if !ok {
		return nil, fmt.Errorf("target is not a channel")
	}

	// Resolve users to invite
	users := make([]tg.InputUserClass, 0)
	for _, u := range p.Users {
		peer, _, err := resolvePeer(ctx, api, u)
		if err != nil {
			continue
		}
		if up, ok := peer.(*tg.InputPeerUser); ok {
			users = append(users, &tg.InputUser{
				UserID:     up.UserID,
				AccessHash: up.AccessHash,
			})
		}
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("no valid users to invite")
	}
	if p.MaxUsers > 0 && len(users) > p.MaxUsers {
		users = users[:p.MaxUsers]
	}

	_, err = api.ChannelsInviteToChannel(ctx, &tg.ChannelsInviteToChannelRequest{
		Channel: &tg.InputChannel{
			ChannelID:  chPeer.ChannelID,
			AccessHash: chPeer.AccessHash,
		},
		Users: users,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"invited": len(users)}, nil
}

// --- Farming ---

func (e *Engine) farming(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Groups    []string `json:"groups"`    // group usernames/links
		Messages  []string `json:"messages"`  // message pool
		Loops     int      `json:"loops"`      // how many times to cycle
		MinDelay  int      `json:"min_delay"`  // seconds
		MaxDelay  int      `json:"max_delay"`  // seconds
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Loops <= 0 {
		p.Loops = 1
	}
	if p.MinDelay <= 0 {
		p.MinDelay = 30
	}
	if p.MaxDelay <= p.MinDelay {
		p.MaxDelay = p.MinDelay + 60
	}
	if len(p.Messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Resolve groups
	type group struct {
		peer  tg.InputPeerClass
		label string
	}
	groups := make([]group, 0)
	for _, g := range p.Groups {
		peer, label, err := resolvePeer(ctx, api, g)
		if err != nil {
			e.logger.Warn("skip group", zap.String("group", g), zap.Error(err))
			continue
		}
		groups = append(groups, group{peer, label})
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("no valid groups resolved")
	}

	sent := 0
	for loop := 0; loop < p.Loops; loop++ {
		for _, g := range groups {
			select {
			case <-ctx.Done():
				return map[string]interface{}{"sent": sent, "loops": loop}, ctx.Err()
			default:
			}

			msg := p.Messages[rand.Intn(len(p.Messages))]
			// Add random noise to avoid pattern detection
			msg = addNoise(msg)

			_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
				Peer:     g.peer,
				Message:  msg,
				RandomID: rand.Int63(),
			})
			if err != nil {
				wait := tgclient.WaitFloodWait(err)
				if wait > 0 {
					e.logger.Warn("flood wait farming",
						zap.String("group", g.label),
						zap.Duration("wait", wait))
					time.Sleep(wait)
					continue
				}
			} else {
				sent++
			}

			// Random delay
			delay := time.Duration(p.MinDelay+rand.Intn(p.MaxDelay-p.MinDelay)) * time.Second
			time.Sleep(delay)
		}
	}

	return map[string]interface{}{"sent": sent, "loops": p.Loops}, nil
}

// --- Scrape Members ---

func (e *Engine) scrapeMembers(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Group string `json:"group"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Limit <= 0 {
		p.Limit = 200
	}

	peer, _, err := resolvePeer(ctx, api, p.Group)
	if err != nil {
		return nil, fmt.Errorf("resolve group: %w", err)
	}
	chPeer, ok := peer.(*tg.InputPeerChannel)
	if !ok {
		return nil, fmt.Errorf("not a channel")
	}

	input := &tg.InputChannel{
		ChannelID:  chPeer.ChannelID,
		AccessHash: chPeer.AccessHash,
	}

	members := make([]map[string]interface{}, 0)
	offset := 0

	for len(members) < p.Limit {
		select {
		case <-ctx.Done():
			return map[string]interface{}{"members": members, "count": len(members)}, ctx.Err()
		default:
		}

		res, err := api.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
			Channel: input,
			Filter:  &tg.ChannelParticipantsRecent{},
			Offset:  offset,
			Limit:   100,
		})
		if err != nil {
			return nil, fmt.Errorf("get participants: %w", err)
		}

		parts, ok := res.(*tg.ChannelsChannelParticipants)
		if !ok {
			break
		}

		for _, u := range parts.Users {
			if user, ok := u.(*tg.User); ok {
				members = append(members, map[string]interface{}{
					"id":         user.ID,
					"username":   user.Username,
					"first_name": user.FirstName,
					"last_name":  user.LastName,
					"phone":      user.Phone,
				})
				if len(members) >= p.Limit {
					break
				}
			}
		}

		if len(parts.Participants) < 100 {
			break
		}
		offset += len(parts.Participants)
	}

	return map[string]interface{}{"members": members, "count": len(members)}, nil
}

// --- Phone Filter ---

func (e *Engine) phoneFilter(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Phones []string `json:"phones"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	contacts := make([]tg.InputPhoneContact, len(p.Phones))
	for i, phone := range p.Phones {
		contacts[i] = tg.InputPhoneContact{
			Phone:    phone,
			ClientID: int64(i),
		}
	}

	res, err := api.ContactsImportContacts(ctx, contacts)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0)

	// Build set of registered user IDs
	registered := make(map[int64]bool)
	for _, u := range res.Users {
		if user, ok := u.(*tg.User); ok {
			registered[user.ID] = true
		}
	}

	for _, imp := range res.Imported {
		phone := p.Phones[imp.ClientID]
		entry := map[string]interface{}{
			"phone":      phone,
			"registered": imp.UserID != 0,
		}
		if imp.UserID != 0 {
			entry["user_id"] = imp.UserID
		}
		result = append(result, entry)
	}

	// Clean up imported contacts
	ids := make([]tg.InputUserClass, 0)
	for _, u := range res.Users {
		if user, ok := u.(*tg.User); ok {
			ids = append(ids, &tg.InputUser{UserID: user.ID, AccessHash: user.AccessHash})
		}
	}
	if len(ids) > 0 {
		api.ContactsDeleteContacts(ctx, ids)
	}

	return map[string]interface{}{
		"results":     result,
		"total":       len(result),
		"registered":  len(registered),
	}, nil
}

// --- Search Groups ---

func (e *Engine) searchGroups(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}

	// Global search via ContactsSearch
	res, err := api.ContactsSearch(ctx, &tg.ContactsSearchRequest{
		Q:     p.Query,
		Limit: p.Limit,
	})
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0)

	for _, chat := range res.Chats {
		switch c := chat.(type) {
		case *tg.Channel:
			results = append(results, map[string]interface{}{
				"id":           c.ID,
				"title":        c.Title,
				"username":     c.Username,
				"members_count": c.ParticipantsCount,
				"megagroup":    c.Megagroup,
				"broadcast":    c.Broadcast,
			})
		case *tg.Chat:
			results = append(results, map[string]interface{}{
				"id":           c.ID,
				"title":        c.Title,
				"members_count": c.ParticipantsCount,
			})
		}
	}

	return map[string]interface{}{"results": results, "total": len(results)}, nil
}

// --- Clone Channel ---

func (e *Engine) cloneChannel(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Source     string `json:"source"`      // source channel username
		Target     string `json:"target"`      // target channel username (must own)
		Limit      int    `json:"limit"`        // max messages to clone
		MediaOnly  bool   `json:"media_only"`   // only clone media messages
		TextOnly   bool   `json:"text_only"`    // only clone text messages
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Limit <= 0 {
		p.Limit = 100
	}

	// Resolve source
	srcPeer, _, err := resolvePeer(ctx, api, p.Source)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	if _, ok := srcPeer.(*tg.InputPeerChannel); !ok {
		return nil, fmt.Errorf("source is not a channel")
	}

	// Resolve target
	tgtPeer, _, err := resolvePeer(ctx, api, p.Target)
	if err != nil {
		return nil, fmt.Errorf("resolve target: %w", err)
	}
	if _, ok := tgtPeer.(*tg.InputPeerChannel); !ok {
		return nil, fmt.Errorf("target is not a channel")
	}

	// Get source messages
	history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  srcPeer,
		Limit: p.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	var messages []tg.MessageClass
	switch h := history.(type) {
	case *tg.MessagesMessages:
		messages = h.Messages
	case *tg.MessagesChannelMessages:
		messages = h.Messages
	case *tg.MessagesMessagesSlice:
		messages = h.Messages
	}

	cloned, skipped := 0, 0
	for _, msg := range messages {
		select {
		case <-ctx.Done():
			return map[string]interface{}{"cloned": cloned, "skipped": skipped}, ctx.Err()
		default:
		}

		m, ok := msg.(*tg.Message)
		if !ok {
			continue
		}

		// Apply filters
		if p.MediaOnly && m.Media == nil {
			skipped++
			continue
		}
		if p.TextOnly && m.Message == "" {
			skipped++
			continue
		}

		// Clone: send as plain text for now (TODO: forward with media)
		if m.Message != "" {
			_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
				Peer:     tgtPeer,
				Message:  m.Message,
				RandomID: rand.Int63(),
			})
			if err != nil {
				wait := tgclient.WaitFloodWait(err)
				if wait > 0 {
					time.Sleep(wait)
					continue
				}
				skipped++
			} else {
				cloned++
			}
		} else {
			skipped++
		}

		// Rate limit
		time.Sleep(time.Duration(1+rand.Intn(3)) * time.Second)
	}

	return map[string]interface{}{"cloned": cloned, "skipped": skipped}, nil
}

// --- Boost (subscribe + views) ---

func (e *Engine) boost(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Channel   string `json:"channel"`     // target channel
		Subscribe bool   `json:"subscribe"`   // join the channel
		ViewPosts int    `json:"view_posts"`  // how many recent posts to view
		Interval  int    `json:"interval"`    // seconds between views
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.ViewPosts <= 0 {
		p.ViewPosts = 10
	}
	if p.Interval <= 0 {
		p.Interval = 5
	}

	// Resolve channel
	peer, _, err := resolvePeer(ctx, api, p.Channel)
	if err != nil {
		return nil, fmt.Errorf("resolve channel: %w", err)
	}
	chPeer, ok := peer.(*tg.InputPeerChannel)
	if !ok {
		return nil, fmt.Errorf("not a channel: %s", p.Channel)
	}
	_ = chPeer

	result := map[string]interface{}{"channel": p.Channel}

	// Step 1: Subscribe (join channel)
	if p.Subscribe {
		// Need AccessHash from Chats in resolve response — use resolve again with full chat info
		res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: strings.TrimPrefix(p.Channel, "@"),
		})
		if err != nil {
			result["subscribe_error"] = err.Error()
		} else {
			for _, chat := range res.Chats {
				if c, ok := chat.(*tg.Channel); ok {
					_, err := api.ChannelsJoinChannel(ctx, &tg.InputChannel{
						ChannelID:  c.ID,
						AccessHash: c.AccessHash,
					})
					if err != nil {
						result["subscribe_error"] = err.Error()
					} else {
						result["subscribed"] = true
					}
					break
				}
			}
		}
		// Small delay after join
		time.Sleep(2 * time.Second)
	}

	// Step 2: View posts (read history = TG counts as view)
	viewed := 0
	offsetID := 0
	for viewed < p.ViewPosts {
		select {
		case <-ctx.Done():
			result["viewed"] = viewed
			return result, ctx.Err()
		default:
		}

		history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    min(100, p.ViewPosts-viewed),
		})
		if err != nil {
			result["view_error"] = err.Error()
			break
		}

		var msgs []tg.MessageClass
		switch h := history.(type) {
		case *tg.MessagesMessages:
			msgs = h.Messages
		case *tg.MessagesChannelMessages:
			msgs = h.Messages
		case *tg.MessagesMessagesSlice:
			msgs = h.Messages
		}

		for _, msg := range msgs {
			if m, ok := msg.(*tg.Message); ok {
				// Read message = increment view count on TG's side
				_, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{
					&tg.InputMessageID{ID: m.ID},
				})
				if err == nil {
					viewed++
				}
				offsetID = m.ID
			}
		}

		if len(msgs) == 0 || len(msgs) < min(100, p.ViewPosts-viewed) {
			break
		}

		// Rate limit
		time.Sleep(time.Duration(p.Interval) * time.Second)
	}

	result["viewed"] = viewed
	return result, nil
}

// --- Red Packet Auto-Grab ---

var (
	rpKeywords = []string{"发送了一个红包", "总金额", "💵", "红包"}
	rpClaimKW  = []string{"领取"}
	rpLockKW   = []string{"解锁", "已锁定"}
)

func (e *Engine) redpacket(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Groups    []string `json:"groups"`    // groups to monitor
		Interval  int      `json:"interval"`  // poll interval seconds
		Duration  int      `json:"duration"`  // total run time seconds (0 = indefinite)
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Interval <= 0 {
		p.Interval = 3
	}
	if p.Duration <= 0 {
		p.Duration = 300 // default 5 min
	}

	// Resolve groups
	type group struct {
		peer  tg.InputPeerClass
		label string
	}
	groups := make([]group, 0)
	for _, g := range p.Groups {
		peer, label, err := resolvePeer(ctx, api, g)
		if err != nil {
			continue
		}
		groups = append(groups, group{peer, label})
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("no valid groups")
	}

	grabbed := 0
	claimed := 0
	deadline := time.Now().Add(time.Duration(p.Duration) * time.Second)
	seenMsg := make(map[int]bool) // dedup per session

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return map[string]interface{}{"grabbed": grabbed, "claimed": claimed}, ctx.Err()
		default:
		}

		for _, g := range groups {
			history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:  g.peer,
				Limit: 20,
			})
			if err != nil {
				continue
			}

			var msgs []tg.MessageClass
			switch h := history.(type) {
			case *tg.MessagesMessages:
				msgs = h.Messages
			case *tg.MessagesChannelMessages:
				msgs = h.Messages
			case *tg.MessagesMessagesSlice:
				msgs = h.Messages
			}

			for _, msg := range msgs {
				m, ok := msg.(*tg.Message)
				if !ok || m.Message == "" {
					continue
				}
				// Dedup
				if seenMsg[m.ID] {
					continue
				}
				seenMsg[m.ID] = true

				// Detect red packet
				if !matchAny(m.Message, rpKeywords) {
					continue
				}
				grabbed++

				// Try to claim via click
				if m.ReplyMarkup != nil {
					if claimedThis := clickCallback(ctx, api, m, g.peer, rpClaimKW, rpLockKW); claimedThis {
						claimed++
					}
				}
			}
		}

		time.Sleep(time.Duration(p.Interval) * time.Second)
	}

	return map[string]interface{}{"grabbed": grabbed, "claimed": claimed}, nil
}

func matchAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func clickCallback(ctx context.Context, api *tg.Client, msg *tg.Message, peer tg.InputPeerClass, claimKW, lockKW []string) bool {
	rm, ok := msg.ReplyMarkup.(*tg.ReplyInlineMarkup)
	if !ok {
		return false
	}
	for _, row := range rm.Rows {
		for _, btn := range row.Buttons {
			cb, ok := btn.(*tg.KeyboardButtonCallback)
			if !ok {
				continue
			}
			label := string(cb.Text)
			// Skip lock buttons
			if matchAny(label, lockKW) {
				continue
			}
			// Click claim button
			if matchAny(label, claimKW) {
				_, err := api.MessagesGetBotCallbackAnswer(ctx, &tg.MessagesGetBotCallbackAnswerRequest{
					Peer: peer,
					MsgID: msg.ID,
					Data:  cb.Data,
				})
				if err == nil {
					return true
				}
			}
		}
	}
	return false
}

// --- Helpers ---

func resolvePeer(ctx context.Context, api *tg.Client, target string) (tg.InputPeerClass, string, error) {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "@") || (!strings.Contains(target, " ") && !strings.HasPrefix(target, "t.me")) {
		username := strings.TrimPrefix(target, "@")
		res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: username,
		})
		if err != nil {
			return nil, target, err
		}
		peer := toInputPeer(res.Peer)
		if peer == nil {
			return nil, target, fmt.Errorf("cannot convert peer")
		}
		return peer, username, nil
	}
	return nil, target, fmt.Errorf("unsupported target: %s", target)
}

// toInputPeer converts an output PeerClass to InputPeerClass
func toInputPeer(p tg.PeerClass) tg.InputPeerClass {
	switch p := p.(type) {
	case *tg.PeerUser:
		return &tg.InputPeerUser{UserID: p.UserID, AccessHash: 0}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		return &tg.InputPeerChannel{ChannelID: p.ChannelID, AccessHash: 0}
	}
	return nil
}

// addNoise adds random spacing/emoji to messages to avoid spam detection
func addNoise(msg string) string {
	emojis := []string{"👍", "🔥", "😊", "💯", "✨", "🙏", "❤️", "👌", "🆗", ""}
	// 50% chance to add trailing emoji
	if rand.Intn(2) == 0 {
		msg += " " + emojis[rand.Intn(len(emojis))]
	}
	// 30% chance to add trailing newline
	if rand.Intn(10) < 3 {
		msg += "\n"
	}
	return msg
}
