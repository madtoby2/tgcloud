package operator

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
)

// resolvePeer resolves a string (username, phone, @username, or numeric ID) to InputPeer
func resolvePeer(ctx context.Context, api *tg.Client, target string) (tg.InputPeerClass, error) {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "@")
	target = strings.TrimPrefix(target, "+")

	if target == "" {
		return nil, fmt.Errorf("empty target")
	}

	// Try as username
	res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: target})
	if err == nil {
		return resolvePeerFromResult(res)
	}

	return nil, fmt.Errorf("failed to resolve: %s", target)
}

func resolvePeerFromResult(res *tg.ContactsResolvedPeer) (tg.InputPeerClass, error) {
	// Check peer directly
	switch p := res.Peer.(type) {
	case *tg.PeerUser:
		for _, u := range res.Users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}, nil
	case *tg.PeerChannel:
		for _, c := range res.Chats {
			if ch, ok := c.(*tg.Channel); ok && ch.ID == p.ChannelID {
				return &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}, nil
			}
		}
	}
	return nil, fmt.Errorf("could not resolve peer")
}

// extractInviteHash gets the hash from a t.me/+XXX or t.me/joinchat/XXX link
func extractInviteHash(link string) string {
	link = strings.TrimSpace(link)
	link = strings.TrimPrefix(link, "https://t.me/")
	link = strings.TrimPrefix(link, "http://t.me/")
	link = strings.TrimPrefix(link, "t.me/")

	if strings.HasPrefix(link, "+") {
		return strings.TrimPrefix(link, "+")
	}
	if strings.HasPrefix(link, "joinchat/") {
		return strings.TrimPrefix(link, "joinchat/")
	}
	// Try as-is
	if len(link) > 5 {
		return link
	}
	return ""
}
