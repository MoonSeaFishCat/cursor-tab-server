package tokenpool

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"cursor-tab-server/internal/coordination"
	"cursor-tab-server/internal/secret"
	"cursor-tab-server/internal/store"
)

const (
	stickyTTL    = 24 * time.Hour
	authCooldown = 5 * time.Minute
	rateCooldown = 30 * time.Second
)

type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Masked     string     `json:"masked"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	Healthy    bool       `json:"healthy"`
	InFlight   int64      `json:"in_flight"`
}

type Lease struct {
	ID    string
	Token string
	pool  *Pool
}

func (l *Lease) TokenValue() string { return l.Token }
func (l *Lease) TokenID() string    { return l.ID }

func (l *Lease) Release(ctx context.Context) {
	if l == nil || l.pool == nil {
		return
	}
	l.pool.coordinator.ReleaseToken(ctx, l.ID)
	l.pool = nil
}

type Pool struct {
	store       *store.Store
	cipher      *secret.Cipher
	coordinator *coordination.Coordinator
}

func New(database *store.Store, cipher *secret.Cipher, coordinator *coordination.Coordinator) *Pool {
	return &Pool{store: database, cipher: cipher, coordinator: coordinator}
}

// Bootstrap imports the legacy token only when the encrypted pool is empty.
func (p *Pool) Bootstrap(ctx context.Context, legacyToken string) error {
	count, err := p.store.CursorTokenCount(ctx)
	if err != nil {
		return err
	}
	if count == 0 && strings.TrimSpace(legacyToken) != "" {
		if _, err := p.Add(ctx, "默认 Token", legacyToken); err != nil {
			return err
		}
	}
	return p.store.DeleteSetting(ctx, store.SettingCursorToken)
}

func (p *Pool) Add(ctx context.Context, name, plaintext string) (Token, error) {
	name = strings.TrimSpace(name)
	plaintext = strings.TrimSpace(plaintext)
	if name == "" {
		return Token{}, fmt.Errorf("token name cannot be empty")
	}
	if len(plaintext) < 10 || len(plaintext) > 4096 {
		return Token{}, fmt.Errorf("token length must be between 10 and 4096")
	}
	ciphertext, nonce, err := p.cipher.Encrypt(plaintext)
	if err != nil {
		return Token{}, err
	}
	now := time.Now().UTC()
	value := store.CursorToken{ID: randomID(), Name: name, Ciphertext: ciphertext, Nonce: nonce, Masked: mask(plaintext), Enabled: true, CreatedAt: now}
	if err := p.store.CreateCursorToken(ctx, value); err != nil {
		return Token{}, err
	}
	return Token{ID: value.ID, Name: value.Name, Masked: value.Masked, Enabled: true, CreatedAt: now, UpdatedAt: now, Healthy: true}, nil
}

func (p *Pool) List(ctx context.Context) ([]Token, error) {
	stored, err := p.store.ListCursorTokens(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(stored))
	for _, value := range stored {
		ids = append(ids, value.ID)
	}
	states := p.coordinator.States(ctx, ids, time.Now().UTC())
	byID := make(map[string]coordination.TokenState, len(states))
	for _, state := range states {
		byID[state.ID] = state
	}
	items := make([]Token, 0, len(stored))
	for _, value := range stored {
		state := byID[value.ID]
		items = append(items, Token{ID: value.ID, Name: value.Name, Masked: value.Masked, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, LastUsedAt: value.LastUsedAt, LastError: value.LastError, Healthy: value.Enabled && state.Healthy, InFlight: state.InFlight})
	}
	return items, nil
}

func (p *Pool) Delete(ctx context.Context, id string) error {
	active, err := p.store.ActiveCursorTokens(ctx)
	if err != nil {
		return err
	}
	for _, token := range active {
		if token.ID == id && len(active) <= 1 {
			return fmt.Errorf("at least one cursor token must remain enabled")
		}
	}
	return p.store.DeleteCursorToken(ctx, id)
}

func (p *Pool) SetEnabled(ctx context.Context, id string, enabled bool) error {
	if enabled {
		return p.store.SetCursorTokenEnabled(ctx, id, true)
	}
	active, err := p.store.ActiveCursorTokens(ctx)
	if err != nil {
		return err
	}
	if len(active) <= 1 {
		return fmt.Errorf("at least one cursor token must remain enabled")
	}
	return p.store.SetCursorTokenEnabled(ctx, id, false)
}

func (p *Pool) Acquire(ctx context.Context, subject, exclude string) (string, string, func(), error) {
	active, err := p.store.ActiveCursorTokens(ctx)
	if err != nil {
		return "", "", nil, err
	}
	ids := make([]string, 0, len(active))
	byID := make(map[string]store.CursorToken, len(active))
	for _, value := range active {
		ids = append(ids, value.ID)
		byID[value.ID] = value
	}
	id, ok := p.coordinator.AcquireToken(ctx, subject, ids, exclude, stickyTTL, time.Now().UTC())
	if !ok {
		return "", "", nil, fmt.Errorf("no healthy cursor token available")
	}
	value := byID[id]
	plaintext, err := p.cipher.Decrypt(value.Ciphertext, value.Nonce)
	if err != nil {
		p.coordinator.ReleaseToken(ctx, id)
		return "", "", nil, err
	}
	release := func() { p.coordinator.ReleaseToken(context.Background(), id) }
	return id, plaintext, release, nil
}

func (p *Pool) MarkSuccess(ctx context.Context, id string) {
	_ = p.store.MarkCursorTokenUsed(context.Background(), id, time.Now().UTC())
}

func (p *Pool) MarkFailure(ctx context.Context, id string, status int) {
	now := time.Now().UTC()
	cooldown := authCooldown
	if status == 429 {
		cooldown = rateCooldown
	}
	p.coordinator.MarkUnhealthy(ctx, id, cooldown, now)
	_ = p.store.MarkCursorTokenError(context.Background(), id, fmt.Sprintf("upstream_status_%d", status), now)
}

func randomID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func mask(value string) string {
	if len(value) <= 8 {
		return "••••"
	}
	return value[:4] + "••••" + value[len(value)-4:]
}
