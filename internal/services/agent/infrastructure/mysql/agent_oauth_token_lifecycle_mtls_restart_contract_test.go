package agentmysql_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// This contract is intentionally opt-in: it starts a disposable MySQL-backed
// Core listener and proves that an exact Runtime retry remains idempotent after
// the listener restarts. It never registers the public callback route.
func TestAgentOAuthTokenLifecycleMySQLMTLSRestartContract(t *testing.T) {
	if os.Getenv("DIPOLE_AGENT_OAUTH_LIFECYCLE_MTLS_RESTART") != "true" {
		t.Skip("OAuth token lifecycle MySQL/mTLS restart contract is disabled")
	}
	ctx := context.Background()
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	repositories, err := sqlcRepository.NewProcessRepositories(db)
	if err != nil {
		t.Fatalf("create OAuth repository composition: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	handoffID := strings.Repeat("h", 22)
	leaseOwner := "oauth-lifecycle-drill"
	handoff := application.AgentOAuthCallbackHandoffV1{
		HandoffUUID: handoffID, TransactionUUID: strings.Repeat("t", 22), OwnerUserUUID: strings.Repeat("u", 22),
		Issuer: "https://issuer.example.test", RedirectURI: "https://dipole.example.test/oauth/callback",
		AuthorizationCodeSHA256: strings.Repeat("a", 64), SealedAuthorizationCode: "v1.nonce.ciphertext.tag",
		RuntimeKeyID: "runtime-key-drill", Status: application.AgentOAuthCallbackHandoffRecordedV1, ExpiresAt: now.Add(5 * time.Minute),
	}
	if created, err := repositories.OAuthCallbackHandoffs.CreateAgentOAuthCallbackHandoff(ctx, handoff); err != nil || !created {
		t.Fatalf("create callback handoff: created=%v err=%v", created, err)
	}
	if claimed, err := repositories.OAuthCallbackHandoffs.ClaimAgentOAuthCallbackHandoff(ctx, handoffID, leaseOwner, now, now.Add(90*time.Second)); err != nil || !claimed {
		t.Fatalf("claim callback handoff: claimed=%v err=%v", claimed, err)
	}

	adapter, err := agentgrpc.NewOAuthCallbackHandoffServer(repositories.OAuthCallbackHandoffs)
	if err != nil {
		t.Fatalf("create OAuth callback Core adapter: %v", err)
	}
	if _, err := adapter.WithOAuthTokenLifecycles(repositories.OAuthTokenLifecycles); err != nil {
		t.Fatalf("attach OAuth token lifecycle repository: %v", err)
	}
	certs := generateReceiptContractCertificates(t)
	request := oauthLifecycleRestartRequest(handoffID, leaseOwner, now.Add(4*time.Minute))

	firstServer := startReceiptContractRPCServer(t, certs, adapter)
	firstConnection := dialReceiptContractRPCClient(t, ctx, certs, firstServer.Address())
	firstClient := agentv1.NewAgentCapabilityServiceClient(firstConnection)
	if response, err := firstClient.PersistOAuthTokenLifecycle(ctx, request); err != nil || response.GetHandoffId() != handoffID || response.GetState() != "active" {
		t.Fatalf("persist lifecycle before restart: response=%+v err=%v", response, err)
	}
	_ = firstConnection.Close()
	firstServer.Close(ctx)

	secondServer := startReceiptContractRPCServer(t, certs, adapter)
	secondConnection := dialReceiptContractRPCClient(t, ctx, certs, secondServer.Address())
	t.Cleanup(func() { _ = secondConnection.Close() })
	secondClient := agentv1.NewAgentCapabilityServiceClient(secondConnection)
	if response, err := secondClient.PersistOAuthTokenLifecycle(ctx, request); err != nil || response.GetHandoffId() != handoffID || response.GetState() != "active" {
		t.Fatalf("exact lifecycle retry after Core restart: response=%+v err=%v", response, err)
	}

	modified := protoCloneOAuthLifecycleRequest(request)
	modified.Scope = "calendar.write"
	if _, err := secondClient.PersistOAuthTokenLifecycle(ctx, modified); status.Code(err) != codes.NotFound {
		t.Fatalf("modified lifecycle retry code=%s err=%v, want not found", status.Code(err), err)
	}
	assertOAuthLifecycleStoredExactlyOnce(t, ctx, db, handoffID, request)
	expiryStore, ok := repositories.OAuthTokenLifecycles.(application.AgentOAuthTokenLifecycleExpiryStoreV1)
	if !ok {
		t.Fatal("OAuth lifecycle repository does not implement expiry maintenance")
	}
	if expired, err := expiryStore.ExpireDueAgentOAuthTokenLifecycles(ctx, now.Add(3*time.Minute), 1); err != nil || expired != 0 {
		t.Fatalf("early lifecycle expiry: expired=%d err=%v", expired, err)
	}
	if expired, err := expiryStore.ExpireDueAgentOAuthTokenLifecycles(ctx, now.Add(5*time.Minute), 1); err != nil || expired != 1 {
		t.Fatalf("due lifecycle expiry: expired=%d err=%v", expired, err)
	}
	assertOAuthLifecycleMaterialExpired(t, ctx, db, handoffID)
}

func oauthLifecycleRestartRequest(handoffID, leaseOwner string, expiresAt time.Time) *agentv1.PersistOAuthTokenLifecycleRequest {
	return &agentv1.PersistOAuthTokenLifecycleRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), HandoffId: handoffID, LeaseOwner: leaseOwner, State: "active",
		SealedTokenBundle: "v1.nonce.ciphertext.tag.wrapped", TokenBundleSha256: strings.Repeat("b", 64),
		AccessTokenExpiresAtUnixMs: expiresAt.UnixMilli(), Scope: "calendar.read",
	}
}

func protoCloneOAuthLifecycleRequest(value *agentv1.PersistOAuthTokenLifecycleRequest) *agentv1.PersistOAuthTokenLifecycleRequest {
	return &agentv1.PersistOAuthTokenLifecycleRequest{
		Context: &commonv1.RequestContext{CallerService: value.GetContext().GetCallerService()}, HandoffId: value.GetHandoffId(), LeaseOwner: value.GetLeaseOwner(),
		State: value.GetState(), SealedTokenBundle: value.GetSealedTokenBundle(), TokenBundleSha256: value.GetTokenBundleSha256(),
		AccessTokenExpiresAtUnixMs: value.GetAccessTokenExpiresAtUnixMs(), Scope: value.GetScope(), RevocationReason: value.GetRevocationReason(),
	}
}

func assertOAuthLifecycleStoredExactlyOnce(t *testing.T, ctx context.Context, db *sql.DB, handoffID string, request *agentv1.PersistOAuthTokenLifecycleRequest) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_oauth_token_lifecycles WHERE handoff_uuid = ?`, handoffID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("lifecycle row count=%d err=%v, want one", count, err)
	}
	var state, sealedBundle, digest, scope string
	var expiresAt time.Time
	if err := db.QueryRowContext(ctx, `SELECT state, sealed_token_bundle, token_bundle_sha256, access_token_expires_at, scope FROM agent_oauth_token_lifecycles WHERE handoff_uuid = ?`, handoffID).
		Scan(&state, &sealedBundle, &digest, &expiresAt, &scope); err != nil {
		t.Fatalf("read persisted lifecycle: %v", err)
	}
	if state != request.GetState() || sealedBundle != request.GetSealedTokenBundle() || digest != request.GetTokenBundleSha256() || scope != request.GetScope() || expiresAt.UnixMilli() != request.GetAccessTokenExpiresAtUnixMs() {
		t.Fatalf("persisted lifecycle drifted: state=%q bundle=%q digest=%q expires=%d scope=%q", state, sealedBundle, digest, expiresAt.UnixMilli(), scope)
	}
}

func assertOAuthLifecycleMaterialExpired(t *testing.T, ctx context.Context, db *sql.DB, handoffID string) {
	t.Helper()
	var state string
	var sealedBundle, digest, scope sql.NullString
	var expiresAt sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT state, sealed_token_bundle, token_bundle_sha256, access_token_expires_at, scope FROM agent_oauth_token_lifecycles WHERE handoff_uuid = ?`, handoffID).
		Scan(&state, &sealedBundle, &digest, &expiresAt, &scope); err != nil {
		t.Fatalf("read expired lifecycle: %v", err)
	}
	if state != "expired" || sealedBundle.Valid || digest.Valid || expiresAt.Valid || scope.Valid {
		t.Fatalf("expired lifecycle retained material: state=%q sealed=%v digest=%v expires=%v scope=%v", state, sealedBundle.Valid, digest.Valid, expiresAt.Valid, scope.Valid)
	}
}
