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

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rpcCoreStub struct{}

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
