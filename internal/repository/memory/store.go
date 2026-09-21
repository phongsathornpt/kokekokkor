package memory

import (
	"context"
	"sync"
	"time"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domaincredential "github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
)

// Store provides thread-safe in-memory implementations of the gateway's persistence contracts.
type Store struct {
	mu            sync.RWMutex
	closed        bool
	catalog       domaincatalog.Snapshot
	credentials   map[domaincredential.Ref][]byte
	oauthTokens   map[string]domainoauth.TokenSet
	responses     map[string]responsestate.Record
	conversations map[string]responsestate.Conversation
}

// New creates a new initialized in-memory Store.
func New() *Store {
	return &Store{
		catalog: domaincatalog.Snapshot{
			Defaults: make(map[provider.Protocol]string),
			Routes:   make(map[string][]domaincatalog.RouteTarget),
		},
		credentials:   make(map[domaincredential.Ref][]byte),
		oauthTokens:   make(map[string]domainoauth.TokenSet),
		responses:     make(map[string]responsestate.Record),
		conversations: make(map[string]responsestate.Conversation),
	}
}

// Close closes the store.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// --- CatalogRepository implementation ---

func (s *Store) Load(_ context.Context) (domaincatalog.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.catalog), nil
}

func (s *Store) Replace(_ context.Context, snapshot domaincatalog.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalog = cloneSnapshot(snapshot)
	return nil
}

// --- CredentialRepository implementation ---

func (s *Store) Get(_ context.Context, ref domaincredential.Ref) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.credentials[ref]
	if !ok {
		return nil, appcredential.ErrNotFound
	}
	return append([]byte(nil), val...), nil
}

func (s *Store) Put(_ context.Context, ref domaincredential.Ref, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.credentials == nil {
		s.credentials = make(map[domaincredential.Ref][]byte)
	}
	s.credentials[ref] = append([]byte(nil), val...)
	return nil
}

func (s *Store) Delete(_ context.Context, ref domaincredential.Ref) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentials, ref)
	return nil
}

// --- OAuth TokenStore implementation ---

type TokenStore struct {
	store *Store
}

func (s *Store) Tokens() *TokenStore {
	return &TokenStore{store: s}
}

func (t *TokenStore) Get(_ context.Context, providerID string) (domainoauth.TokenSet, error) {
	t.store.mu.RLock()
	defer t.store.mu.RUnlock()
	tokens, ok := t.store.oauthTokens[providerID]
	if !ok {
		return domainoauth.TokenSet{}, appoauth.ErrTokenNotFound
	}
	return tokens, nil
}

func (t *TokenStore) Put(_ context.Context, providerID string, tokens domainoauth.TokenSet) error {
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	if t.store.oauthTokens == nil {
		t.store.oauthTokens = make(map[string]domainoauth.TokenSet)
	}
	t.store.oauthTokens[providerID] = tokens
	return nil
}

func (t *TokenStore) Delete(_ context.Context, providerID string) error {
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	delete(t.store.oauthTokens, providerID)
	return nil
}

// --- ResponseStateStore implementation ---

func (s *Store) LoadResponse(_ context.Context, id string) (responsestate.Record, error) {
	s.mu.RLock()
	record, ok := s.responses[id]
	s.mu.RUnlock()
	if !ok {
		return responsestate.Record{}, responsestate.ErrNotFound
	}
	if !record.ExpiresAt.IsZero() && !record.ExpiresAt.After(time.Now()) {
		s.mu.Lock()
		delete(s.responses, id)
		s.mu.Unlock()
		return responsestate.Record{}, responsestate.ErrNotFound
	}
	return cloneRecord(record), nil
}

func (s *Store) SaveResponse(_ context.Context, id string, record responsestate.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.responses == nil {
		s.responses = make(map[string]responsestate.Record)
	}
	s.responses[id] = cloneRecord(record)
	return nil
}

func (s *Store) DeleteResponse(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.responses, id)
	return nil
}

// --- ConversationStore implementation ---

func (s *Store) LoadConversation(_ context.Context, id string) (responsestate.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.conversations[id]
	if !ok {
		return responsestate.Conversation{}, responsestate.ErrNotFound
	}
	return cloneConversation(conv), nil
}

func (s *Store) SaveConversation(_ context.Context, conv responsestate.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conversations == nil {
		s.conversations = make(map[string]responsestate.Conversation)
	}
	s.conversations[conv.ID] = cloneConversation(conv)
	return nil
}

func (s *Store) DeleteConversation(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conversations, id)
	return nil
}

func cloneSnapshot(s domaincatalog.Snapshot) domaincatalog.Snapshot {
	out := domaincatalog.Snapshot{
		Providers: make([]domaincatalog.Provider, len(s.Providers)),
		Defaults:  make(map[provider.Protocol]string, len(s.Defaults)),
		Routes:    make(map[string][]domaincatalog.RouteTarget, len(s.Routes)),
	}
	copy(out.Providers, s.Providers)
	for k, v := range s.Defaults {
		out.Defaults[k] = v
	}
	for k, v := range s.Routes {
		targets := make([]domaincatalog.RouteTarget, len(v))
		copy(targets, v)
		out.Routes[k] = targets
	}
	return out
}

func cloneRecord(r responsestate.Record) responsestate.Record {
	out := r
	out.Payload = append([]byte(nil), r.Payload...)
	return out
}

func cloneConversation(c responsestate.Conversation) responsestate.Conversation {
	out := c
	if c.Metadata != nil {
		out.Metadata = make(map[string]string, len(c.Metadata))
		for k, v := range c.Metadata {
			out.Metadata[k] = v
		}
	}
	return out
}
