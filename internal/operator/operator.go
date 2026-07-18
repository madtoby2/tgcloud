package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/telegram/uploader"
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
	OpAutoReply  = "autoreply"
	OpWarmup     = "warmup"
	OpScrapeMsgs = "scrape_scripts"
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
	case OpAutoReply:
		return e.autoreply(ctx, api, op.Params)
	case OpWarmup:
		return e.warmup(ctx, api, op.Params)
	case OpScrapeMsgs:
		return e.scrapeScripts(ctx, api, op.Params)
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
		Targets  []string `json:"targets"`
		Message  string   `json:"message"`
		Media    string   `json:"media"`    // path to media file (local path)
		Medias   []string `json:"medias"`   // multiple media files
		Interval int      `json:"interval"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Interval <= 0 { p.Interval = 5 }

	sent, failed := 0, 0

	// Collect all media paths
	mediaPaths := make([]string, 0)
	if p.Media != "" { mediaPaths = append(mediaPaths, p.Media) }
	mediaPaths = append(mediaPaths, p.Medias...)

	// Upload media files once (reuse across targets)
	up := uploader.NewUploader(api)
	var uploadedMedia []tg.InputMediaClass
	if len(mediaPaths) > 0 {
		for _, mp := range mediaPaths {
			media, err := uploadMedia(ctx, up, mp)
			if err != nil {
				e.logger.Warn("upload failed", zap.String("file", mp), zap.Error(err))
				continue
			}
			uploadedMedia = append(uploadedMedia, media)
		}
	}

	// Resolve targets
	type target struct {
		peer  tg.InputPeerClass
		label string
	}
	targets := make([]target, 0)
	for _, t := range p.Targets {
		peer, label, err := resolvePeer(ctx, api, t)
		if err != nil { failed++; continue }
		targets = append(targets, target{peer, label})
	}

	// Send to each target
	for _, t := range targets {
		select {
		case <-ctx.Done(): return nil, ctx.Err()
		default:
		}

		if len(uploadedMedia) > 0 {
			// Send media (one media per target from the pool)
			media := uploadedMedia[rand.Intn(len(uploadedMedia))]
			_, err := api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
				Peer:     t.peer,
				Media:    media,
				Message:  p.Message,
				RandomID: rand.Int63(),
			})
			if err != nil {
				if wait := tgclient.WaitFloodWait(err); wait > 0 {
					time.Sleep(wait); continue
				}
				failed++
			} else { sent++ }
		} else if p.Message != "" {
			_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
				Peer: t.peer, Message: p.Message, RandomID: rand.Int63(),
			})
			if err != nil {
				if wait := tgclient.WaitFloodWait(err); wait > 0 {
					time.Sleep(wait); continue
				}
				failed++
			} else { sent++ }
		}

		base := time.Duration(p.Interval) * time.Second
		jitter := time.Duration(rand.Intn(p.Interval)) * time.Second
		time.Sleep(base + jitter)
	}

	return map[string]interface{}{"sent": sent, "failed": failed, "media_uploaded": len(uploadedMedia)}, nil
}

// uploadMedia detects type by extension, uploads and returns InputMedia
func uploadMedia(ctx context.Context, up *uploader.Uploader, path string) (tg.InputMediaClass, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()

	stat, err := f.Stat()
	if err != nil { return nil, err }

	upload := uploader.NewUpload(filepath.Base(path), f, stat.Size())
	file, err := up.Upload(ctx, upload)
	if err != nil { return nil, err }

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp":
		return &tg.InputMediaUploadedPhoto{File: file}, nil
	case ".gif", ".mp4":
		doc := &tg.InputMediaUploadedDocument{
			File: file,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeVideo{RoundMessage: ext == ".mp4", SupportsStreaming: true},
			},
			MimeType: mimeByExt(ext),
		}
		return doc, nil
	case ".webm", ".mkv", ".avi", ".mov":
		return &tg.InputMediaUploadedDocument{
			File: file,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeVideo{SupportsStreaming: true},
			},
			MimeType: mimeByExt(ext),
		}, nil
	case ".mp3", ".ogg", ".wav", ".flac", ".opus":
		return &tg.InputMediaUploadedDocument{
			File: file,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeAudio{Duration: 0, Title: filepath.Base(path)},
			},
			MimeType: mimeByExt(ext),
		}, nil
	case ".pdf", ".zip", ".rar", ".7z", ".doc", ".docx", ".txt", ".apk":
		return &tg.InputMediaUploadedDocument{
			File:      file,
			MimeType:  mimeByExt(ext),
			Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: filepath.Base(path)}},
		}, nil
	default:
		// Generic document
		return &tg.InputMediaUploadedDocument{
			File:      file,
			Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: filepath.Base(path)}},
		}, nil
	}
}

func mimeByExt(ext string) string {
	switch ext {
	case ".jpg", ".jpeg": return "image/jpeg"
	case ".png": return "image/png"
	case ".webp": return "image/webp"
	case ".gif": return "image/gif"
	case ".mp4": return "video/mp4"
	case ".webm": return "video/webm"
	case ".mkv": return "video/x-matroska"
	case ".mp3": return "audio/mpeg"
	case ".ogg": return "audio/ogg"
	case ".opus": return "audio/opus"
	case ".pdf": return "application/pdf"
	case ".zip": return "application/zip"
	case ".apk": return "application/vnd.android.package-archive"
	default: return "application/octet-stream"
	}
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
		Groups    []string `json:"groups"`
		Messages  []string `json:"messages"`  // message pool
		Loops     int      `json:"loops"`
		MinDelay  int      `json:"min_delay"`
		MaxDelay  int      `json:"max_delay"`
		Mode      string   `json:"mode"`      // "solo" or "conversation"
		Personas  int      `json:"personas"`  // how many fake personas (conversation mode)
		ReplyGap  int      `json:"reply_gap"` // seconds between replies in conversation
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Loops <= 0 { p.Loops = 1 }
	if p.MinDelay <= 0 { p.MinDelay = 30 }
	if p.MaxDelay <= p.MinDelay { p.MaxDelay = p.MinDelay + 60 }
	if p.ReplyGap <= 0 { p.ReplyGap = 5 }
	if p.Personas <= 0 { p.Personas = 3 }
	if len(p.Messages) == 0 { return nil, fmt.Errorf("no messages provided") }

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

			if p.Mode == "conversation" {
				// Multi-persona reply chain in one group
				for persona := 0; persona < p.Personas; persona++ {
					select {
					case <-ctx.Done():
						return map[string]interface{}{"sent": sent, "loops": loop}, ctx.Err()
					default:
					}
					msg := p.Messages[rand.Intn(len(p.Messages))]
					msg = addNoise(msg)
					// Add reply-to-like prefix (simulate conversational turn)
					if persona > 0 && persona%2 == 1 {
						// "Reply" feel: add agreement prefix
						prefixes := []string{"确实 ", "是的 ", "哈哈 ", "对啊 ", "👍 ", ""}
						msg = prefixes[rand.Intn(len(prefixes))] + msg
					}
					_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
						Peer:     g.peer,
						Message:  msg,
						RandomID: rand.Int63(),
					})
					if err != nil {
						wait := tgclient.WaitFloodWait(err)
						if wait > 0 {
							time.Sleep(wait)
							continue
						}
					} else {
						sent++
					}
					// Short gap between personas
					time.Sleep(time.Duration(p.ReplyGap) * time.Second)
				}
			} else {
				// Solo mode: one message per group
				msg := p.Messages[rand.Intn(len(p.Messages))]
				msg = addNoise(msg)
				_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
					Peer:     g.peer,
					Message:  msg,
					RandomID: rand.Int63(),
				})
				if err != nil {
					wait := tgclient.WaitFloodWait(err)
					if wait > 0 {
						time.Sleep(wait)
						continue
					}
				} else {
					sent++
				}
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
	rpKeywords  = []string{"发送了一个红包", "总金额", "💵", "红包"}
	rpClaimKW   = []string{"领取"}
	rpLockKW    = []string{"解锁", "已锁定"}
	rpMathExpr  = regexp.MustCompile(`([\d一二三四五六七八九十百千万亿零]+)\s*([+\-×xX*÷/])\s*([\d一二三四五六七八九十百千万亿零]+)\s*[=＝]\s*[?？]`)
	cnDigits    = map[rune]int64{'一':1,'二':2,'三':3,'四':4,'五':5,'六':6,'七':7,'八':8,'九':9,'十':10,'百':100,'千':1000,'万':10000,'亿':100000000,'零':0}
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

				// Try claim: 1) direct "领取" click  2) captcha solve
				if m.ReplyMarkup != nil {
					if claimedThis := clickCallback(ctx, api, m, g.peer, rpClaimKW, rpLockKW); claimedThis {
						claimed++
						continue
					}
					// Try captcha
					if claimedThis := solveAndClick(ctx, api, m, g.peer); claimedThis {
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

// solveAndClick finds math expression in text, computes answer, clicks matching button
func solveAndClick(ctx context.Context, api *tg.Client, msg *tg.Message, peer tg.InputPeerClass) bool {
	text := msg.Message
	m := rpMathExpr.FindStringSubmatch(text)
	if m == nil {
		return false
	}
	a := parseCNNumber(m[1])
	op := m[2]
	b := parseCNNumber(m[3])

	var answer int64
	switch op {
	case "+":
		answer = a + b
	case "-":
		answer = a - b
	case "×", "x", "X", "*":
		answer = a * b
	case "÷", "/":
		if b != 0 {
			answer = a / b
		}
	default:
		return false
	}

	// Find button with matching label
	rm, ok := msg.ReplyMarkup.(*tg.ReplyInlineMarkup)
	if !ok {
		return false
	}
	answerStr := fmt.Sprintf("%d", answer)
	for _, row := range rm.Rows {
		for _, btn := range row.Buttons {
			cb, ok := btn.(*tg.KeyboardButtonCallback)
			if !ok {
				continue
			}
			label := cleanButtonText(string(cb.Text))
			if label == answerStr {
				_, err := api.MessagesGetBotCallbackAnswer(ctx, &tg.MessagesGetBotCallbackAnswerRequest{
					Peer:  peer,
					MsgID: msg.ID,
					Data:  cb.Data,
				})
				return err == nil
			}
		}
	}
	return false
}

// parseCNNumber converts Chinese or Arabic digit string to int64
func parseCNNumber(s string) int64 {
	s = strings.TrimSpace(s)
	if n, ok := tryParseInt(s); ok {
		return n
	}
	// Chinese digits
	runes := []rune(s)
	var result, section, num int64
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if val, ok := cnDigits[ch]; ok {
			switch val {
			case 10:
				if num == 0 { num = 1 }
				section += num * 10
			case 100:
				if num == 0 { num = 1 }
				section += num * 100
			case 1000:
				if num == 0 { num = 1 }
				section += num * 1000
			case 10000:
				if section == 0 { section = 1 }
				result += section * 10000
				section = 0
			case 100000000:
				if section == 0 { section = 1 }
				result += section * 100000000
				section = 0
			default:
				num = val
			}
		}
	}
	return result + section
}

func tryParseInt(s string) (int64, bool) {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int64(ch-'0')
	}
	return n, true
}

func cleanButtonText(s string) string {
	// Strip zero-width chars and emoji
	var out []rune
	for _, ch := range s {
		if ch <= 0x200D && ch >= 0x200B { continue } // zero-width
		if ch >= 0x1F300 && ch <= 0x1F9FF { continue } // emoji
		out = append(out, ch)
	}
	return strings.TrimSpace(string(out))
}

// --- Auto-Reply Engine ---

type replyRule struct {
	Keyword  string `json:"keyword"`
	Response string `json:"response"`
	Match    string `json:"match"` // contains, exact, regex
}

func (e *Engine) autoreply(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Targets  []string    `json:"targets"` // groups/DMs to monitor
		Rules    []replyRule `json:"rules"`    // keyword→response mapping
		Interval int         `json:"interval"` // poll sec
		Duration int         `json:"duration"` // total sec
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if len(p.Rules) == 0 {
		return nil, fmt.Errorf("no rules configured")
	}
	if p.Interval <= 0 { p.Interval = 5 }
	if p.Duration <= 0 { p.Duration = 300 }

	// Resolve targets
	var peers []tg.InputPeerClass
	for _, t := range p.Targets {
		peer, _, err := resolvePeer(ctx, api, t)
		if err != nil {
			continue
		}
		peers = append(peers, peer)
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("no valid targets")
	}

	replied := 0
	seen := make(map[int]bool)
	deadline := time.Now().Add(time.Duration(p.Duration) * time.Second)
	cooldown := make(map[string]time.Time) // key = target:user → last reply

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return map[string]interface{}{"replied": replied}, ctx.Err()
		default:
		}

		for _, peer := range peers {
			history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:  peer,
				Limit: 30,
			})
			if err != nil { continue }

			var msgs []tg.MessageClass
			switch h := history.(type) {
			case *tg.MessagesMessages: msgs = h.Messages
			case *tg.MessagesChannelMessages: msgs = h.Messages
			case *tg.MessagesMessagesSlice: msgs = h.Messages
			}

			for _, msg := range msgs {
				m, ok := msg.(*tg.Message)
				if !ok || m.Message == "" || seen[m.ID] { continue }
				seen[m.ID] = true

				// Check cooldown per user (60s)
				senderKey := fmt.Sprintf("%d", getSenderID(m))
				if t, ok := cooldown[senderKey]; ok && time.Since(t) < 60*time.Second {
					continue
				}

				for _, rule := range p.Rules {
					if matchRule(m.Message, rule) {
						_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
							Peer:     peer,
							Message:  rule.Response,
							RandomID: rand.Int63(),
						})
						if err != nil {
							wait := tgclient.WaitFloodWait(err)
							if wait > 0 { time.Sleep(wait); continue }
						} else {
							replied++
							cooldown[senderKey] = time.Now()
						}
						break // one rule per message
					}
				}
			}
		}
		time.Sleep(time.Duration(p.Interval) * time.Second)
	}
	return map[string]interface{}{"replied": replied}, nil
}

func matchRule(text string, rule replyRule) bool {
	switch rule.Match {
	case "exact":
		return text == rule.Keyword
	case "regex":
		re, err := regexp.Compile(rule.Keyword)
		if err != nil { return false }
		return re.MatchString(text)
	default: // contains
		return strings.Contains(strings.ToLower(text), strings.ToLower(rule.Keyword))
	}
}

func getSenderID(m *tg.Message) int64 {
	if p, ok := m.FromID.(*tg.PeerUser); ok { return p.UserID }
	return 0
}

// --- Account Warmup ---

func (e *Engine) warmup(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Channels  []string `json:"channels"`  // channels to browse
		Duration  int      `json:"duration"`  // total sec
		Interval  int      `json:"interval"`  // sec between actions
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Duration <= 0 { p.Duration = 600 }
	if p.Interval <= 0 { p.Interval = 30 }

	// Resolve channels
	var peers []tg.InputPeerClass
	for _, c := range p.Channels {
		peer, _, err := resolvePeer(ctx, api, c)
		if err != nil { continue }
		peers = append(peers, peer)
	}
	if len(peers) == 0 {
		// Use default dialogs
		dialogs, _ := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{Limit: 10})
		if d, ok := dialogs.(*tg.MessagesDialogsSlice); ok {
			for _, dl := range d.Dialogs {
				if dp, ok := dl.(*tg.Dialog); ok {
					peers = append(peers, toInputPeer(dp.Peer))
				}
			}
		}
	}
	if len(peers) == 0 {
		peers = append(peers, &tg.InputPeerUser{})
	}

	actions := 0
	deadline := time.Now().Add(time.Duration(p.Duration) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return map[string]interface{}{"actions": actions}, ctx.Err()
		default:
		}

		peer := peers[rand.Intn(len(peers))]
		action := rand.Intn(3)

		switch action {
		case 0: // Browse messages
			api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:  peer,
				Limit: 10,
			})
		case 1: // Send typing indicator (simulate reading)
			api.MessagesSetTyping(ctx, &tg.MessagesSetTypingRequest{
				Peer:   peer,
				Action: &tg.SendMessageTypingAction{},
			})
		case 2: // Get dialogs (simulate normal activity)
			api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
				Limit: 5,
			})
		}
		actions++

		jitter := time.Duration(rand.Intn(p.Interval)) * time.Second
		time.Sleep(time.Duration(p.Interval)*time.Second + jitter)
	}

	return map[string]interface{}{"actions": actions}, nil
}

// --- Script Scraper ---

func (e *Engine) scrapeScripts(ctx context.Context, api *tg.Client, params json.RawMessage) (interface{}, error) {
	var p struct {
		Group    string `json:"group"`
		Limit    int    `json:"limit"`
		MinLen   int    `json:"min_len"`  // minimum message length
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Limit <= 0 { p.Limit = 200 }
	if p.MinLen <= 0 { p.MinLen = 5 }

	peer, _, err := resolveInputPeer(ctx, api, p.Group)
	if err != nil {
		return nil, fmt.Errorf("resolve group: %w", err)
	}

	scripts := make([]string, 0)
	seen := make(map[string]bool)
	offsetID := 0

	for len(scripts) < p.Limit {
		select {
		case <-ctx.Done():
			return map[string]interface{}{"scripts": scripts, "count": len(scripts)}, ctx.Err()
		default:
		}

		history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    min(100, p.Limit - len(scripts)),
		})
		if err != nil { break }

		var msgs []tg.MessageClass
		switch h := history.(type) {
		case *tg.MessagesMessages: msgs = h.Messages
		case *tg.MessagesChannelMessages: msgs = h.Messages
		case *tg.MessagesMessagesSlice: msgs = h.Messages
		}

		for _, msg := range msgs {
			m, ok := msg.(*tg.Message)
			if !ok || m.Message == "" { continue }
			text := strings.TrimSpace(m.Message)
			if len([]rune(text)) < p.MinLen { continue }
			// Dedup
			if seen[text] { continue }
			seen[text] = true
			scripts = append(scripts, text)
			offsetID = m.ID
			if len(scripts) >= p.Limit { break }
		}

		if len(msgs) < 50 { break }
	}

	return map[string]interface{}{"scripts": scripts, "count": len(scripts)}, nil
}

// --- Helpers ---

// resolveInputPeer resolves a target string to a proper InputPeer with correct AccessHash.
func resolveInputPeer(ctx context.Context, api *tg.Client, target string) (tg.InputPeerClass, string, error) {
	target = strings.TrimSpace(target)
	username := strings.TrimPrefix(target, "@")
	username = strings.TrimPrefix(username, "https://t.me/")
	username = strings.TrimPrefix(username, "t.me/")

	res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, target, err
	}

	switch p := res.Peer.(type) {
	case *tg.PeerUser:
		for _, u := range res.Users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, username, nil
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}, username, nil
	case *tg.PeerChannel:
		for _, c := range res.Chats {
			if ch, ok := c.(*tg.Channel); ok && ch.ID == p.ChannelID {
				return &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}, username, nil
			}
		}
	}
	return nil, target, fmt.Errorf("cannot resolve peer: %s", target)
}

// resolvePeer is legacy — prefers resolveInputPeer for proper access hashes
func resolvePeer(ctx context.Context, api *tg.Client, target string) (tg.InputPeerClass, string, error) {
	return resolveInputPeer(ctx, api, target)
}

// toInputPeer converts an output PeerClass to InputPeerClass (legacy, use resolveInputPeer instead)
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
