package tgclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

type SessionStore interface {
	SaveSession(phone string, data []byte) error
	GetSession(phone string) ([]byte, error)
}

type StatusUpdate func(status string, floodWait int64)

type Client struct {
	phone      string
	apiID      int
	apiHash    string
	sessionDir string
	storage    SessionStore
	client     *telegram.Client
	api        *tg.Client
	onStatus   StatusUpdate

	mu     sync.Mutex
	logger *zap.Logger
}

func New(phone string, apiID int, apiHash, sessionDir string, storage SessionStore, logger *zap.Logger) *Client {
	return &Client{
		phone:      phone,
		apiID:      apiID,
		apiHash:    apiHash,
		sessionDir: sessionDir,
		storage:    storage,
		logger:     logger,
	}
}

func (c *Client) SetStatusCallback(fn StatusUpdate) { c.onStatus = fn }

func (c *Client) Connect(ctx context.Context, codeFn func() (string, error), passFn func() (string, error)) error {
	sessionStorage := &dbSessionStorage{
		phone:   c.phone,
		storage: c.storage,
	}
	c.client = telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: sessionStorage,
	})
	return c.client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(codeAuth{
			phone:  c.phone,
			codeFn: codeFn,
			passFn: passFn,
		}, auth.SendCodeOptions{})
		if err := c.client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}
		c.api = c.client.API()
		c.updateStatus("online", 0)

		// fetch self info
		self, err := c.client.Self(ctx)
		if err == nil {
			c.logger.Info("connected", zap.String("phone", c.phone),
				zap.String("name", self.FirstName),
				zap.String("username", self.Username))
		}

		if c.onStatus != nil {
			c.onStatus("online", 0)
		}
		<-ctx.Done()
		return ctx.Err()
	})
}

func (c *Client) API() *tg.Client { return c.api }

func (c *Client) updateStatus(status string, floodWait int64) {
	if c.onStatus != nil {
		c.onStatus(status, floodWait)
	}
}

// --- Auth ---

type codeAuth struct {
	phone  string
	codeFn func() (string, error)
	passFn func() (string, error)
}

func (c codeAuth) Phone(_ context.Context) (string, error)                       { return c.phone, nil }
func (c codeAuth) Password(_ context.Context) (string, error)                    { return c.passFn() }
func (c codeAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error { return nil }
func (c codeAuth) SignUp(_ context.Context) (auth.UserInfo, error)               { return auth.UserInfo{}, nil }

func (c codeAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	code, err := c.codeFn()
	if err != nil {
		return "", err
	}
	return code, nil
}

// --- DB-backed session ---

type dbSessionStorage struct {
	phone   string
	storage SessionStore
}

func (d *dbSessionStorage) LoadSession(_ context.Context) ([]byte, error) {
	data, err := d.storage.GetSession(d.phone)
	if err != nil {
		return nil, session.ErrNotFound
	}
	return data, nil
}

func (d *dbSessionStorage) StoreSession(_ context.Context, data []byte) error {
	return d.storage.SaveSession(d.phone, data)
}

// --- Helpers ---

func WaitFloodWait(err error) time.Duration {
	d, ok := telegram.AsFloodWait(err)
	if !ok {
		return 0
	}
	return d
}

func IsFloodWait(err error) bool {
	_, ok := telegram.AsFloodWait(err)
	return ok
}

func FormatFloodWait(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
