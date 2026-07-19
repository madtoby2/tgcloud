package tgclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/session"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

type Client struct {
	phone      string
	apiID      int
	apiHash    string
	sessionDir string

	client *telegram.Client
	api    *tg.Client

	logger *zap.Logger

	onStatus func(status string, floodWait int64)
}

func New(phone string, apiID int, apiHash string, sessionDir string, logger *zap.Logger) *Client {
	return &Client{
		phone:      phone,
		apiID:      apiID,
		apiHash:    apiHash,
		sessionDir: sessionDir,
		logger:     logger,
	}
}

func (c *Client) SetStatusCallback(fn func(status string, floodWait int64)) {
	c.onStatus = fn
}

func (c *Client) API() *tg.Client {
	if c.api == nil && c.client != nil {
		c.api = c.client.API()
	}
	return c.api
}

func (c *Client) SessionPath() string {
	return filepath.Join(c.sessionDir, c.phone+".session")
}

func (c *Client) Connect(ctx context.Context, codeFn func() (string, error), passFn func() (string, error)) error {
	os.MkdirAll(c.sessionDir, 0700)
	sessionPath := c.SessionPath()

	storage := &session.FileStorage{Path: sessionPath}

	c.client = telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: storage,
	})

	return c.client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(codeAuth{
			phone:  c.phone,
			codeFn: codeFn,
			passFn: passFn,
		}, auth.SendCodeOptions{})

		if err := c.client.Auth().IfNecessary(ctx, flow); err != nil {
			c.setStatus("error", 0)
			return err
		}

		c.api = c.client.API()

		// Fetch self info
		self, err := c.client.Self(ctx)
		if err != nil {
			c.setStatus("error", 0)
			return err
		}

		// Store user info for caller to use
		if c.onStatus != nil {
			c.onStatus(fmt.Sprintf("online|%s|%s|%d", self.FirstName, self.Username, self.ID), 0)
		}
		c.setStatus("online", 0)

		<-ctx.Done()
		return ctx.Err()
	})
}

func (c *Client) setStatus(status string, floodWait int64) {
	if c.onStatus != nil {
		c.onStatus(status, floodWait)
	}
}

// Auth flow implementation
type codeAuth struct {
	phone  string
	codeFn func() (string, error)
	passFn func() (string, error)
}

func (c codeAuth) Phone(_ context.Context) (string, error)     { return c.phone, nil }
func (c codeAuth) Password(_ context.Context) (string, error)  { return c.passFn() }
func (c codeAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error { return nil }
func (c codeAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{FirstName: "tgcloud", LastName: "User"}, nil
}

func (c codeAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return c.codeFn()
}

// ---- helpers ----

func RandomInt63() int64 {
	b := make([]byte, 8)
	rand.Read(b)
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}

// WaitFloodWait extracts flood wait duration from gotd errors.
func WaitFloodWait(err error) time.Duration {
	d, ok := telegram.AsFloodWait(err)
	if !ok {
		return 0
	}
	return d
}

// IsFloodWait checks if error is a flood wait error.
func IsFloodWait(err error) bool {
	_, ok := telegram.AsFloodWait(err)
	return ok
}
