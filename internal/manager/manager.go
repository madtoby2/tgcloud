package manager

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/madtoby2/tgcloud/internal/store"
	"github.com/madtoby2/tgcloud/internal/tgclient"
	"go.uber.org/zap"
)

// PendingLogin holds channels for the auth flow
type PendingLogin struct {
	Phone    string
	CodeChan chan string
	PassChan chan string
	ErrChan  chan error
	DoneChan chan struct{}
}

type connectedClient struct {
	client    *tgclient.Client
	accountID int64
	cancel    context.CancelFunc
}

// Manager is the account pool + orchestration
type Manager struct {
	store    *store.Store
	logger   *zap.Logger
	apiID    int
	apiHash  string

	mu         sync.RWMutex
	clients    map[int64]*connectedClient
	pending    map[string]*PendingLogin
	connecting atomic.Int64
}

func New(s *store.Store, apiID int, apiHash string, logger *zap.Logger) *Manager {
	return &Manager{
		store:   s,
		logger:  logger,
		apiID:   apiID,
		apiHash: apiHash,
		clients: make(map[int64]*connectedClient),
		pending: make(map[string]*PendingLogin),
	}
}

func (m *Manager) AccountStore() *store.AccountStore {
	return store.NewAccountStore(m.store.DB())
}

// AddAccount adds an account to the DB
func (m *Manager) AddAccount(phone, proxy string) (int64, error) {
	a, err := m.AccountStore().Create(phone, proxy)
	if err != nil {
		return 0, err
	}
	return a.ID, nil
}

// RequestLogin starts the authentication flow for a phone number
func (m *Manager) RequestLogin(phone string) (*PendingLogin, error) {
	// Find or create account
	astore := m.AccountStore()
	a, err := astore.GetByPhone(phone)
	if err != nil {
		a, err = astore.Create(phone, "")
		if err != nil {
			return nil, err
		}
	}

	p := &PendingLogin{
		Phone:    phone,
		CodeChan: make(chan string, 1),
		PassChan: make(chan string, 1),
		ErrChan:  make(chan error, 1),
		DoneChan: make(chan struct{}),
	}

	m.mu.Lock()
	m.pending[phone] = p
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	client := tgclient.New(phone, m.apiID, m.apiHash, "./data/sessions", m.logger)
	client.SetStatusCallback(func(status string, floodWait int64) {
		// status can be "online", "error", or "online|First|user|12345"
		if len(status) > 6 && status[:6] == "online" {
			astore.Update(a.ID, map[string]interface{}{
				"status":         "online",
				"last_login_at":  time.Now().UTC().Format("2006-01-02 15:04:05"),
			})
			// Parse user info from status string: "online|FirstName|username|userid"
			parts := splitStatus(status)
			if len(parts) >= 2 {
				astore.Update(a.ID, map[string]interface{}{"first_name": parts[1]})
			}
			if len(parts) >= 3 {
				astore.Update(a.ID, map[string]interface{}{"username": parts[2]})
			}
			if len(parts) >= 4 {
				var uid int64
				fmt.Sscanf(parts[3], "%d", &uid)
				astore.Update(a.ID, map[string]interface{}{"user_id": uid})
			}
		} else {
			astore.Update(a.ID, map[string]interface{}{"status": status})
		}
	})

	cc := &connectedClient{
		client:    client,
		accountID: a.ID,
		cancel:    cancel,
	}

	m.mu.Lock()
	m.clients[a.ID] = cc
	m.mu.Unlock()

	go func() {
		err := client.Connect(ctx,
			func() (string, error) {
				select {
				case code := <-p.CodeChan:
					return code, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
			func() (string, error) {
				select {
				case pass := <-p.PassChan:
					return pass, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
		)

		m.mu.Lock()
		delete(m.pending, phone)
		m.mu.Unlock()

		if err != nil && err != context.Canceled {
			astore.Update(a.ID, map[string]interface{}{"status": "error"})
			p.ErrChan <- err
		}
		close(p.DoneChan)
	}()

	return p, nil
}

func (m *Manager) SubmitCode(phone, code string) error {
	m.mu.RLock()
	p, ok := m.pending[phone]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no pending login for %s", phone)
	}
	select {
	case p.CodeChan <- code:
		return nil
	default:
		return fmt.Errorf("code already submitted")
	}
}

func (m *Manager) SubmitPassword(phone, password string) error {
	m.mu.RLock()
	p, ok := m.pending[phone]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no pending login for %s", phone)
	}
	select {
	case p.PassChan <- password:
		return nil
	default:
		return fmt.Errorf("password already submitted")
	}
}

func (m *Manager) GetPending(phone string) *PendingLogin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pending[phone]
}

// GetClient returns the tgclient for an account if it's connected
func (m *Manager) GetClient(id int64) *tgclient.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.clients[id]; ok {
		return c.client
	}
	return nil
}

// LogoutAccount disconnects and removes session
func (m *Manager) LogoutAccount(id int64) error {
	m.mu.Lock()
	if c, ok := m.clients[id]; ok {
		c.cancel()
		delete(m.clients, id)
	}
	m.mu.Unlock()

	astore := m.AccountStore()
	astore.Update(id, map[string]interface{}{"status": "offline"})
	// Delete session file
	return nil
}

// ImportSession imports a base64-encoded session
func (m *Manager) ImportSession(phone string, b64data string) (*store.Account, error) {
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	astore := m.AccountStore()
	a, err := astore.GetByPhone(phone)
	if err != nil {
		a, err = astore.Create(phone, "")
		if err != nil {
			return nil, err
		}
	}
	store.NewCategoryStore(m.store.DB()).SaveSession(a.ID, data, m.store.DB())
	return a, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.cancel()
	}
}

func splitStatus(s string) []string {
	parts := make([]string, 0)
	start := 0
	sep := byte('|')
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
