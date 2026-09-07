package app

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"soteria/internal/domain"
	"soteria/internal/infra/keychain"
	"soteria/internal/infra/store"
	"soteria/internal/infra/webdav"
)

var ErrPassword = errors.New("password required")

// Session owns the current login; every other use case asks it for the client.
type Session struct {
	mu           sync.Mutex
	c            *webdav.Client
	cur          *domain.Server
	OnConnect    func(*webdav.Client)
	OnDisconnect func()
}

func (s *Session) Client() (*webdav.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.c == nil {
		return nil, errors.New("not connected")
	}
	return s.c, nil
}

func (s *Session) Current() *domain.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// Use swaps the active client; Connect calls it, tests call it directly.
func (s *Session) Use(c *webdav.Client, sv *domain.Server) {
	s.mu.Lock()
	s.c, s.cur = c, sv
	s.mu.Unlock()
}

func (s *Session) Servers() ([]domain.Server, error) { return store.LoadServers() }

func (s *Session) Connect(sv domain.Server, password string) (*domain.Server, error) {
	c, err := webdav.New(sv, password)
	if err != nil {
		return nil, err
	}
	if err := c.Ping(context.Background()); err != nil {
		return nil, err
	}
	if sv.ID == "" {
		sv.ID = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	if sv.Name == "" {
		sv.Name = c.Base.Host
	}
	sv.LastUsed = time.Now().Unix()
	if err := store.SaveServer(sv); err != nil {
		return nil, err
	}
	if sv.Remember {
		_ = keychain.Set(sv.ID, password)
	} else {
		keychain.Delete(sv.ID)
	}
	s.Use(c, &sv)
	if s.OnConnect != nil {
		s.OnConnect(c)
	}
	return &sv, nil
}

func (s *Session) ConnectSaved(id string) (*domain.Server, error) {
	sv, ok := store.FindServer(id)
	if !ok {
		return nil, errors.New("server not found")
	}
	pw, err := keychain.Get(id)
	if err != nil || pw == "" {
		return nil, ErrPassword
	}
	return s.Connect(sv, pw)
}

// SignOut disconnects and forgets the saved password; Disconnect alone keeps it for next time.
func (s *Session) SignOut() {
	if cur := s.Current(); cur != nil {
		keychain.Delete(cur.ID)
	}
	s.Disconnect()
}

func (s *Session) Disconnect() {
	if s.OnDisconnect != nil {
		s.OnDisconnect()
	}
	s.Use(nil, nil)
}

func (s *Session) Forget(id string) error {
	keychain.Delete(id)
	return store.RemoveServer(id)
}
