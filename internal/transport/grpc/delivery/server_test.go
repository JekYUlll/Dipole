package deliverygrpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	deliverycontract "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type recordingSink struct {
	mu      sync.Mutex
	batches []*deliveryv1.NodeDeliveryBatch
	started chan struct{}
	release chan struct{}
}

func (s *recordingSink) Observe(batch *deliveryv1.NodeDeliveryBatch) {
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.release != nil {
		<-s.release
	}
	s.mu.Lock()
	s.batches = append(s.batches, batch)
	s.mu.Unlock()
}

func (s *recordingSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.batches)
}

func TestShadowServerObservesAndDeduplicatesWithoutDelivery(t *testing.T) {
	sink := &recordingSink{}
	server, err := NewShadowServer("gateway-1", 4, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)

	batch := validNodeBatch("NB1", "gateway-1")
	observation, err := server.ObserveNodeBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if err := deliverycontract.ValidateNodeObservation(observation); err != nil {
		t.Fatalf("validate observation: %v", err)
	}
	if observation.Status != deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_OBSERVED ||
		observation.ObservedItems != 1 || observation.ObservedConnections != 2 || observation.Duplicate {
		t.Fatalf("unexpected observation: %+v", observation)
	}

	duplicate, err := server.ObserveNodeBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Duplicate {
		t.Fatal("expected duplicate batch observation")
	}
	eventually(t, func() bool { return sink.count() == 1 })
}

func TestShadowServerRejectsWrongNode(t *testing.T) {
	sink := &recordingSink{}
	server, err := NewShadowServer("gateway-1", 2, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)

	observation, err := server.ObserveNodeBatch(context.Background(), validNodeBatch("NB2", "gateway-2"))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Status != deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_REJECTED ||
		observation.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INVALID_ITEM {
		t.Fatalf("unexpected rejection: %+v", observation)
	}
	if sink.count() != 0 {
		t.Fatal("wrong-node batch reached observation sink")
	}
}

func TestShadowServerReportsBoundedBackpressure(t *testing.T) {
	sink := &recordingSink{started: make(chan struct{}, 1), release: make(chan struct{})}
	server, err := NewShadowServer("gateway-1", 1, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(sink.release)
		server.Close()
	})

	if _, err := server.ObserveNodeBatch(context.Background(), validNodeBatch("NB1", "gateway-1")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("observation worker did not start")
	}
	if _, err := server.ObserveNodeBatch(context.Background(), validNodeBatch("NB2", "gateway-1")); err != nil {
		t.Fatal(err)
	}
	observation, err := server.ObserveNodeBatch(context.Background(), validNodeBatch("NB3", "gateway-1"))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Status != deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_BACKPRESSURED ||
		observation.Pressure == nil || observation.Pressure.Depth != 1 || observation.Pressure.Capacity != 1 {
		t.Fatalf("unexpected backpressure: %+v", observation)
	}
	if err := deliverycontract.ValidateNodeObservation(observation); err != nil {
		t.Fatalf("validate backpressure: %v", err)
	}
}

func TestShadowServerConcurrentCloseDoesNotPanic(t *testing.T) {
	sink := &recordingSink{}
	server, err := NewShadowServer("gateway-1", 8, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for index := 0; index < 100; index++ {
			_, _ = server.ObserveNodeBatch(context.Background(), validNodeBatch(time.Now().String(), "gateway-1"))
		}
	}()
	server.Close()
	<-done
	if _, err := server.ObserveNodeBatch(context.Background(), validNodeBatch("closed", "gateway-1")); err == nil {
		t.Fatal("expected closed receiver to reject requests")
	}
}

func TestShadowServerRejectsUnsafeConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		nodeID     string
		capacity   int
		retryAfter time.Duration
		sink       ObservationSink
	}{
		{name: "missing node", capacity: 1, retryAfter: time.Millisecond, sink: &recordingSink{}},
		{name: "missing capacity", nodeID: "gateway-1", retryAfter: time.Millisecond, sink: &recordingSink{}},
		{name: "missing retry", nodeID: "gateway-1", capacity: 1, sink: &recordingSink{}},
		{name: "overflowing retry", nodeID: "gateway-1", capacity: 1, retryAfter: time.Minute + time.Millisecond, sink: &recordingSink{}},
		{name: "missing sink", nodeID: "gateway-1", capacity: 1, retryAfter: time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewShadowServer(tt.nodeID, tt.capacity, tt.retryAfter, tt.sink); err == nil {
				t.Fatal("expected invalid observation receiver configuration to fail")
			}
		})
	}
}

func TestShadowServerRequiresRealtimeServiceAuthentication(t *testing.T) {
	authorized := newObservationClient(t, grpcauth.Credentials{Service: "dipole-realtime", Secret: "test-secret"})
	if _, err := authorized.ObserveNodeBatch(context.Background(), validNodeBatch("authorized", "gateway-1")); err != nil {
		t.Fatalf("authorized observation: %v", err)
	}

	denied := newObservationClient(t, grpcauth.Credentials{Service: "dipole-message", Secret: "test-secret"})
	if _, err := denied.ObserveNodeBatch(context.Background(), validNodeBatch("denied", "gateway-1")); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected caller rejection, got %v", err)
	}
}

func validNodeBatch(batchID, nodeID string) *deliveryv1.NodeDeliveryBatch {
	return &deliveryv1.NodeDeliveryBatch{
		ContractVersion: "v1", BatchId: batchID, TargetNodeId: nodeID, SourceEventId: "E1",
		CreatedAt: timestamppb.New(time.Unix(1, 0).UTC()),
		Items: []*deliveryv1.NodeDeliveryItem{{
			DeliveryId: "D-" + batchID, RecipientUserId: "U1", ConnectionIds: []string{"C1", "C2"},
			EventType: "message.direct.created", PayloadJson: []byte(`{"message_id":"M1"}`),
			OrderingKey: "direct:U1:U2", Mode: deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT,
		}},
	}
}

func eventually(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}

func newObservationClient(t *testing.T, credentials grpcauth.Credentials) deliveryv1.NodeDeliveryServiceClient {
	t.Helper()
	interceptor, err := grpcauth.NewUnaryServerInterceptor("test-secret", "dipole-realtime")
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewShadowServer("gateway-1", 4, 25*time.Millisecond, &recordingSink{})
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	deliveryv1.RegisterNodeDeliveryServiceServer(server, receiver)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		receiver.Close()
		_ = listener.Close()
	})
	clientInterceptor, err := grpcauth.NewUnaryClientInterceptor(credentials)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := grpc.NewClient(
		"passthrough:///delivery-observation-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithUnaryInterceptor(clientInterceptor),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return deliveryv1.NewNodeDeliveryServiceClient(connection)
}
