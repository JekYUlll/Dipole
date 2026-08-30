package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

// lazyCoreCapability lets Message start before Core is listening. Failed
// connections are not retained, so the next request or readiness probe retries.
type lazyCoreCapability struct {
	cfg config.InternalRPC

	mu     sync.Mutex
	client *coregrpc.Client
	conn   *grpc.ClientConn
	closed bool
}

var _ application.CoreCapability = (*lazyCoreCapability)(nil)

func newLazyCoreCapability(cfg config.InternalRPC) *lazyCoreCapability {
	return &lazyCoreCapability{cfg: cfg}
}

func lazyCoreCapabilityReadinessProbe(name string, capability *lazyCoreCapability) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{
		Name:                  name,
		RequireInitialSuccess: true,
		Check: func(ctx context.Context) error {
			if capability == nil {
				return errors.New("lazy Core capability is unavailable")
			}
			return capability.Check(ctx)
		},
	}
}

func (c *lazyCoreCapability) resolve(ctx context.Context) (*coregrpc.Client, *grpc.ClientConn, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil, errors.New("lazy Core capability is closed")
	}
	if c.client != nil && c.conn != nil {
		client, conn := c.client, c.conn
		c.mu.Unlock()
		return client, conn, nil
	}
	c.mu.Unlock()

	client, conn, err := dialCoreCapability(ctx, c.cfg)
	if err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		_ = conn.Close()
		return nil, nil, errors.New("lazy Core capability is closed")
	}
	if c.client != nil && c.conn != nil {
		_ = conn.Close()
		return c.client, c.conn, nil
	}
	c.client, c.conn = client, conn
	return client, conn, nil
}

func dialCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := platformrpc.Dial(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{
		Service: messageServiceName,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClientForService(corev1.NewCoreCapabilityServiceClient(connection), messageServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func (c *lazyCoreCapability) Check(ctx context.Context) error {
	_, conn, err := c.resolve(ctx)
	if err != nil {
		return err
	}
	response, err := healthv1.NewHealthClient(conn).Check(ctx, &healthv1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if response.GetStatus() != healthv1.HealthCheckResponse_SERVING {
		return errors.New("Core capability is not serving")
	}
	return nil
}

func (c *lazyCoreCapability) GetUserByUUID(userUUID string) (*model.User, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.GetUserByUUID(userUUID)
}

func (c *lazyCoreCapability) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return false, err
	}
	return client.CanSendDirectMessage(userUUID, friendUUID)
}

func (c *lazyCoreCapability) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.GetGroupByUUID(groupUUID)
}

func (c *lazyCoreCapability) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.GetGroupMember(groupUUID, userUUID)
}

func (c *lazyCoreCapability) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.ListGroupMembers(groupUUID)
}

func (c *lazyCoreCapability) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.GetOwnedFile(uploaderUUID, fileUUID)
}

func (c *lazyCoreCapability) ListOwnedFiles(uploaderUUID, beforeFileUUID string, limit int) (*application.OwnedFilePage, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.ListOwnedFiles(uploaderUUID, beforeFileUUID, limit)
}

func (c *lazyCoreCapability) ListSearchConversationKeys(userUUID string) ([]string, error) {
	client, _, err := c.resolve(context.Background())
	if err != nil {
		return nil, err
	}
	return client.ListSearchConversationKeys(userUUID)
}

func (c *lazyCoreCapability) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	c.client, c.conn = nil, nil
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}
