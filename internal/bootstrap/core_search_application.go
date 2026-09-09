package bootstrap

import (
	"context"
	"errors"
	"sync"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	searchgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/search"
	"google.golang.org/grpc"
)

// lazyCoreSearchApplication keeps Core startup independent from Search startup.
type lazyCoreSearchApplication struct {
	cfg    config.InternalRPC
	mu     sync.Mutex
	client *searchgrpc.Client
	conn   *grpc.ClientConn
}

var _ application.SearchApplication = (*lazyCoreSearchApplication)(nil)

func newLazyCoreSearchApplication(cfg config.InternalRPC) *lazyCoreSearchApplication {
	return &lazyCoreSearchApplication{cfg: cfg}
}

func (s *lazyCoreSearchApplication) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.Search(principal, text, limit)
}

func (s *lazyCoreSearchApplication) getClient() (*searchgrpc.Client, error) {
	if s == nil {
		return nil, errors.New("Core Search application is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	client, conn, err := DialCoreSearchApplication(context.Background(), s.cfg)
	if err != nil {
		return nil, err
	}
	s.client, s.conn = client, conn
	return client, nil
}

func (s *lazyCoreSearchApplication) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.client, s.conn = nil, nil
	return err
}
