package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/madtoby2/tgcloud/internal/operator"
	"github.com/madtoby2/tgcloud/internal/store"
	"github.com/madtoby2/tgcloud/internal/tgclient"
	"go.uber.org/zap"
)

type PendingCode struct {
	Phone     string
	CodeChan  chan string
	PassChan  chan string
	ErrChan   chan error
	DoneChan  chan struct{}
}

type connectedClient struct {
	client   *tgclient.Client
	accountID int64
	cancel   context.CancelFunc
}

type Manager struct {
	store    *store.Store
	logger   *zap.Logger
	apiID    int
	apiHash   string
	engine   *operator.Engine

	mu          sync.RWMutex
	clients     map[int64]*connectedClient
	pending     map[string]*PendingCode
	connecting  atomic.Int64
}

func New(s *store.Store, apiID int, apiHash string, logger *zap.Logger) *Manager {
	return &Manager{
		store:   s,
		logger:  logger,
		apiID:   apiID,
		apiHash: apiHash,
		engine:  operator.New(s, logger),
		clients: make(map[int64]*connectedClient),
		pending: make(map[string]*PendingCode),
	}
}

func (m *Manager) Engine() *operator.Engine { return m.engine }

func (m *Manager) AddAccount(phone, proxy string) (*store.Account, error) {
	return m.store.CreateAccount(phone, proxy)
}

func (m *Manager) GetAccount(id int64) (*store.Account, error) {
	return m.store.GetAccount(id)
}

func (m *Manager) ListAccounts() ([]*store.Account, error) {
	return m.store.ListAccounts()
}

func (m *Manager) DeleteAccount(id int64) error {
	m.mu.Lock()
	if c, ok := m.clients[id]; ok {
		c.cancel()
		delete(m.clients, id)
	}
	m.mu.Unlock()
	return m.store.DeleteAccount(id)
}

func (m *Manager) RequestLogin(phone string) (*PendingCode, error) {
	account, err := m.AddAccount(phone, "")
	if err != nil {
		return nil, err
	}

	p := &PendingCode{
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
	client := tgclient.New(phone, m.apiID, m.apiHash, "", m.store, m.logger)
	client.SetStatusCallback(func(status string, floodWait int64) {
		m.store.UpdateAccountStatus(account.ID, status, floodWait)
	})

	cc := &connectedClient{
		client:   client,
		accountID: account.ID,
		cancel:   cancel,
	}

	m.mu.Lock()
	m.clients[account.ID] = cc
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
			m.store.UpdateAccountStatus(account.ID, "error", 0)
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
		return fmt.Errorf("2fa password already submitted")
	}
}

func (m *Manager) GetPendingCode(phone string) *PendingCode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pending[phone]
}

func (m *Manager) GetClientAPI(id int64) *tgclient.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.clients[id]; ok {
		return c.client
	}
	return nil
}

// --- Operations ---

func (m *Manager) CreateOperation(accountID int64, opType string, params json.RawMessage) (*store.Operation, error) {
	op, err := m.store.CreateOperation(accountID, opType, params)
	if err != nil {
		return nil, err
	}

	// Auto-execute if account is online
	cli := m.GetClientAPI(accountID)
	if cli != nil {
		api := cli.API()
		if api != nil {
			m.engine.Execute(op, api)
		}
	}

	return op, nil
}

func (m *Manager) ListOperations(accountID int64) ([]*store.Operation, error) {
	return m.store.ListOperations(accountID)
}

func (m *Manager) CancelOperation(id int64) {
	m.engine.Cancel(id)
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.cancel()
	}
}
