package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type rpcCoreStub struct{}

type rpcAgentCapabilityStub struct{ application.AgentCapabilityV1 }

func (rpcAgentCapabilityStub) ListConversations(context.Context, application.AgentInvocationV1, int) ([]*model.Conversation, error) {
	return []*model.Conversation{{ConversationKey: "direct:U100:UAI"}}, nil
}

type rpcAgentResolverStub struct{}

func (rpcAgentResolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return application.AgentInvocationV1{PrincipalUUID: "U100", AgentUUID: "UAI"}, nil
}

type rpcAgentAdmissionStub struct{}

func (rpcAgentAdmissionStub) Admit(context.Context, application.AgentRunAdmissionRequestV1) (*application.AgentRunAdmissionV1, error) {
	return &application.AgentRunAdmissionV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", RunStatus: application.AgentRunStatusRunning}, nil
}

func (rpcAgentAdmissionStub) Complete(context.Context, string, string, string, string) error {
	return nil
}

func (rpcAgentAdmissionStub) Finish(context.Context, string, string, string, string, application.AgentRunStatusV1, string) error {
	return nil
}

func (rpcCoreStub) ListSearchConversationKeys(userUUID string) ([]string, error) {
	return []string{"direct:" + userUUID + ":U2"}, nil
}

type rpcSearchStub struct{}

type rpcSyncStub struct{}

func (rpcSyncStub) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	return &application.SyncPage{
		Items:   []*model.SyncMessage{{SyncSeq: afterSeq + 1, ConversationKey: "direct:" + userUUID + ":U2"}},
		NextSeq: afterSeq + 1,
	}, nil
}

func (rpcSyncStub) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: 9}, nil
}

func (rpcSyncStub) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: syncSeq}, nil
}

func (rpcSyncStub) ListGroupCheckpoints(string, string, []string) ([]*model.GroupSyncCheckpoint, error) {
	return []*model.GroupSyncCheckpoint{{GroupUUID: "G1", LatestMessageSeq: 12}}, nil
}

func (rpcSyncStub) AdvanceGroupCheckpoint(_, _, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error) {
	return &model.GroupSyncCheckpoint{GroupUUID: groupUUID, PulledMessageSeq: messageSeq}, nil
}

func (rpcSearchStub) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	return []*model.MessageSearchDocument{{
		MessageUUID: "M1", ConversationKey: "direct:" + principal + ":U2", MessageSeq: 7,
		Revision: 1, SenderUUID: "U2", MessageType: model.MessageTypeText, Content: text, SentAt: time.Unix(1, 0),
	}}, nil
}

func (rpcCoreStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID, Nickname: "RPC User"}, nil
}
func (rpcCoreStub) CanSendDirectMessage(string, string) (bool, error) { return true, nil }
func (rpcCoreStub) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return &model.Group{UUID: groupUUID, Name: "RPC Group"}, nil
}
func (rpcCoreStub) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID}, nil
}
func (rpcCoreStub) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return []*model.GroupMember{{GroupUUID: groupUUID, UserUUID: "U1"}}, nil
}
func (rpcCoreStub) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	return &model.UploadedFile{UUID: fileUUID, UploaderUUID: uploaderUUID, FileName: "rpc-file"}, nil
}

func TestCoreRPCServerAndClientUseAuthenticatedNetworkChannel(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})

	cfg.CoreTarget = server.Address()
	client, connection, err := DialCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	user, err := client.GetUserByUUID("U100")
	if err != nil {
		t.Fatalf("get user through core rpc: %v", err)
	}
	if user == nil || user.UUID != "U100" || user.Nickname != "RPC User" {
		t.Fatalf("unexpected user: %#v", user)
	}
	_, err = corev1.NewCoreCapabilityServiceClient(connection).GetUser(context.Background(), &corev1.GetUserRequest{
		Context: &commonv1.RequestContext{CallerService: "dipole-gateway"},
		UserId:  "U100",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected authenticated caller mismatch rejection, got %v", err)
	}
}

func TestAgentRPCUsesAuthenticatedLeastPrivilegeChannel(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
	server, err := NewCoreRPCServerWithAgent(cfg, rpcCoreStub{}, rpcAgentCapabilityStub{}, rpcAgentResolverStub{}, rpcAgentAdmissionStub{})
	if err != nil {
		t.Fatalf("start Agent rpc server: %v", err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	interceptor, err := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: agentServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		t.Fatalf("create Agent rpc credentials: %v", err)
	}
	connection, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(interceptor))
	if err != nil {
		t.Fatalf("dial Agent rpc: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	agentClient := agentv1.NewAgentCapabilityServiceClient(connection)
	response, err := agentClient.AdmitRun(context.Background(), &agentv1.AdmitRunRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TenantId: "dipole", PrincipalUserId: "U100",
		AgentId: "UAI", TriggerType: "message.direct.created", TriggerRef: "M100", RuntimeId: agentServiceName, Mode: "shadow",
	})
	if err != nil || response.GetRunId() != "RUN-1" {
		t.Fatalf("admit Agent Run through authenticated channel: response=%+v err=%v", response, err)
	}
	finished, err := agentClient.FinishRun(context.Background(), &agentv1.FinishRunRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", RunId: "RUN-1", RunStatus: "cancelled",
	})
	if err != nil || finished.GetRunStatus() != "cancelled" {
		t.Fatalf("finish Agent Run through authenticated channel: response=%+v err=%v", finished, err)
	}
	_, err = corev1.NewCoreCapabilityServiceClient(connection).GetUser(context.Background(), &corev1.GetUserRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, UserId: "U100",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent Core method code = %s, want permission denied", status.Code(err))
	}
}

func TestGatewayUsesItsOwnAuthenticatedCoreIdentity(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})
	cfg.CoreTarget = server.Address()
	client, connection, err := DialGatewayCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial gateway core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	user, err := client.GetUserByUUID("U-GATEWAY")
	if err != nil || user == nil || user.UUID != "U-GATEWAY" {
		t.Fatalf("gateway core query failed: user=%+v err=%v", user, err)
	}
}

func TestSearchServiceUsesAuthenticatedCoreAndGatewayChannels(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0",
		SearchListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2,
	}
	coreServer, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start Core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		coreServer.Close(ctx)
	})
	cfg.CoreTarget = coreServer.Address()
	core, coreConnection, err := DialSearchCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Core as Search service: %v", err)
	}
	t.Cleanup(func() { _ = coreConnection.Close() })
	keys, err := core.ListSearchConversationKeys("U1")
	if err != nil || len(keys) != 1 || keys[0] != "direct:U1:U2" {
		t.Fatalf("Search Core scope: keys=%v err=%v", keys, err)
	}
	if _, err := core.GetUserByUUID("U1"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected Search identity to be denied unrelated Core capability, got %v", err)
	}

	searchServer, err := NewSearchRPCServer(cfg, rpcSearchStub{})
	if err != nil {
		t.Fatalf("start Search rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		searchServer.Close(ctx)
	})
	cfg.SearchTarget = searchServer.Address()
	search, searchConnection, err := DialSearchApplication(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Search application: %v", err)
	}
	t.Cleanup(func() { _ = searchConnection.Close() })
	documents, err := search.Search("U1", "migration", 10)
	if err != nil || len(documents) != 1 || documents[0].Content != "migration" {
		t.Fatalf("Search rpc result: documents=%+v err=%v", documents, err)
	}
}

func TestSyncServiceUsesAuthenticatedCoreAndCoreChannels(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0",
		SyncListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2,
	}
	coreServer, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start Core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		coreServer.Close(ctx)
	})
	cfg.CoreTarget = coreServer.Address()
	core, coreConnection, err := DialSyncCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Core as Sync service: %v", err)
	}
	t.Cleanup(func() { _ = coreConnection.Close() })
	member, err := core.GetGroupMember("G1", "U1")
	if err != nil || member == nil || member.GroupUUID != "G1" {
		t.Fatalf("Sync Core membership scope: member=%+v err=%v", member, err)
	}
	if _, err := core.GetUserByUUID("U1"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected Sync identity to be denied unrelated Core capability, got %v", err)
	}

	syncServer, err := NewSyncRPCServer(cfg, rpcSyncStub{})
	if err != nil {
		t.Fatalf("start Sync rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		syncServer.Close(ctx)
	})
	cfg.SyncTarget = syncServer.Address()
	syncApplication, syncConnection, err := DialCoreSyncApplication(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Sync application as Core: %v", err)
	}
	t.Cleanup(func() { _ = syncConnection.Close() })
	page, err := syncApplication.List("U1", 7, 20)
	if err != nil || page == nil || page.NextSeq != 8 || len(page.Items) != 1 {
		t.Fatalf("Sync rpc result: page=%+v err=%v", page, err)
	}
	checkpoint, err := syncApplication.GetCheckpoint("U1", "web-1")
	if err != nil || checkpoint == nil || checkpoint.SyncSeq != 9 || checkpoint.DeviceID != "web-1" {
		t.Fatalf("Sync checkpoint result: checkpoint=%+v err=%v", checkpoint, err)
	}
}

func TestInternalRPCRejectsMissingRuntimeCredentials(t *testing.T) {
	if _, err := NewCoreRPCServer(config.InternalRPC{Enabled: true, CoreListenAddress: "127.0.0.1:0"}, rpcCoreStub{}); err == nil {
		t.Fatal("expected core rpc server without shared secret to fail")
	}
	if _, _, err := DialCoreCapability(context.Background(), config.InternalRPC{Enabled: true, CoreTarget: "127.0.0.1:1"}); err == nil {
		t.Fatal("expected core rpc client without shared secret to fail")
	}
}

func TestCoreRPCServerAndClientUseMutualTLS(t *testing.T) {
	certFile, keyFile, caFile := writeRPCIdentity(t, messageServiceName)
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
		TLSEnabled:         true,
		TLSCertFile:        certFile,
		TLSKeyFile:         keyFile,
		TLSCAFile:          caFile,
		TLSServerName:      "localhost",
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start mtls core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})
	cfg.CoreTarget = server.Address()
	client, connection, err := DialCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial mtls core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if user, err := client.GetUserByUUID("U-TLS"); err != nil || user == nil || user.UUID != "U-TLS" {
		t.Fatalf("mtls core call failed: user=%+v err=%v", user, err)
	}
}

func TestInternalRPCRejectsPlaintextOutsideLoopback(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "0.0.0.0:0"}
	if _, err := NewCoreRPCServer(cfg, rpcCoreStub{}); err == nil {
		t.Fatal("expected non-loopback plaintext listener to fail")
	}
	cfg.CoreTarget = "10.0.0.1:9091"
	if _, _, err := DialCoreCapability(context.Background(), cfg); err == nil {
		t.Fatal("expected non-loopback plaintext target to fail")
	}
}

func writeRPCIdentity(t testing.TB, serviceName string) (string, string, string) {
	t.Helper()
	directory := t.TempDir()
	now := time.Now().UTC()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Dipole Test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create ca certificate: %v", err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: serviceName},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	certFile := filepath.Join(directory, "identity.pem")
	keyFile := filepath.Join(directory, "identity-key.pem")
	caFile := filepath.Join(directory, "ca.pem")
	writePEM(t, certFile, "CERTIFICATE", leafDER, 0o644)
	writePEM(t, keyFile, "EC PRIVATE KEY", keyDER, 0o600)
	writePEM(t, caFile, "CERTIFICATE", caDER, 0o644)
	return certFile, keyFile, caFile
}

func writePEM(t testing.TB, path, blockType string, data []byte, mode os.FileMode) {
	t.Helper()
	encoded := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data})
	if err := os.WriteFile(path, encoded, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
