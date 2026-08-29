package syncgrpc

import (
	"context"
	"net"
	"testing"

	syncv1 "github.com/JekYUlll/Dipole/api/gen/go/sync/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubSyncApplication struct {
	t *testing.T
}

func (s stubSyncApplication) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	if userUUID != "U1" || afterSeq != 100 || limit != 200 {
		s.t.Fatalf("unexpected sync query: user=%q after=%d limit=%d", userUUID, afterSeq, limit)
	}
	return &application.SyncPage{
		Items: []*model.SyncMessage{
			{SyncSeq: 101, ConversationKey: "direct:U1:U2", MessageUUID: "M7", MessageSeq: 8, Message: &model.Message{ID: 7, UUID: "M7", Seq: 8, SenderUUID: "U2"}},
			nil,
		},
		NextSeq: 101,
		HasMore: true,
	}, nil
}

func (s stubSyncApplication) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: 10}, nil
}

func (s stubSyncApplication) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: syncSeq}, nil
}

func (s stubSyncApplication) ListGroupCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error) {
	return []*model.GroupSyncCheckpoint{{GroupUUID: groupUUIDs[0], LatestMessageSeq: 12, LatestMessageUUID: "M12", PulledMessageSeq: 9}}, nil
}

func (s stubSyncApplication) AdvanceGroupCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error) {
	return &model.GroupSyncCheckpoint{GroupUUID: groupUUID, LatestMessageSeq: 12, LatestMessageUUID: "M12", PulledMessageSeq: messageSeq}, nil
}

func TestRemoteClientImplementsSyncApplication(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubSyncApplication{t: t})
	client, err := NewClient(rpc)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	page, err := client.List("U1", 100, 500)
	if err != nil {
		t.Fatalf("list sync messages: %v", err)
	}
	if page.NextSeq != 101 || !page.HasMore || len(page.Items) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	if page.Items[0].Message == nil || page.Items[0].Message.UUID != "M7" || page.Items[0].Message.Seq != 8 {
		t.Fatalf("unexpected message mapping: %+v", page.Items[0])
	}
	if page.Items[0].MessageUUID != "M7" || page.Items[0].MessageSeq != 8 {
		t.Fatalf("unexpected Sync locator mapping: %+v", page.Items[0])
	}
	checkpoint, err := client.GetCheckpoint("U1", "web-1")
	if err != nil || checkpoint.DeviceID != "web-1" || checkpoint.SyncSeq != 10 {
		t.Fatalf("get remote checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	checkpoint, err = client.AdvanceCheckpoint("U1", "web-1", 9)
	if err != nil || checkpoint.SyncSeq != 9 {
		t.Fatalf("advance remote checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	groups, err := client.ListGroupCheckpoints("U1", "web-1", []string{"G1"})
	if err != nil || len(groups) != 1 || groups[0].LatestMessageSeq != 12 || groups[0].PulledMessageSeq != 9 {
		t.Fatalf("list remote group checkpoints: checkpoints=%+v err=%v", groups, err)
	}
	group, err := client.AdvanceGroupCheckpoint("U1", "web-1", "G1", 11)
	if err != nil || group.GroupUUID != "G1" || group.PulledMessageSeq != 11 {
		t.Fatalf("advance remote group checkpoint: checkpoint=%+v err=%v", group, err)
	}
}

func TestNewClientForServiceRequiresCallerIdentity(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubSyncApplication{t: t})
	if _, err := NewClientForService(rpc, "  "); err == nil {
		t.Fatal("expected empty caller service to fail")
	}
}

func TestServerRejectsMissingPrincipal(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubSyncApplication{t: t})
	_, err := rpc.ListSyncMessages(context.Background(), &syncv1.ListSyncMessagesRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestServerRejectsMissingCheckpointDevice(t *testing.T) {
	rpc := newBufconnRPCClient(t, stubSyncApplication{t: t})
	_, err := rpc.GetDeviceCheckpoint(context.Background(), &syncv1.GetDeviceCheckpointRequest{
		Context: grpccommon.RequestContext("U1", "dipole-gateway"),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func newBufconnRPCClient(t *testing.T, application stubSyncApplication) syncv1.SyncQueryServiceClient {
	t.Helper()
	adapter, err := NewServer(application)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	syncv1.RegisterSyncQueryServiceServer(server, adapter)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	connection, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatalf("new grpc client: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return syncv1.NewSyncQueryServiceClient(connection)
}
