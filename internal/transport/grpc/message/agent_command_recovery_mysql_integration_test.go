package messagegrpc

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
	_ "github.com/go-sql-driver/mysql"
)

type activeMessageCommandUserFinder struct{}

func (activeMessageCommandUserFinder) GetByUUID(uuid string) (*model.User, error) {
	switch uuid {
	case "U100", "UAI":
		return &model.User{UUID: uuid, Status: model.UserStatusNormal}, nil
	default:
		return nil, nil
	}
}

func TestCoreAgentMessageCommandRecoversOneMySQLCommitAfterGRPCResponseLoss(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_AGENT_MESSAGE_COMMAND_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_AGENT_MESSAGE_COMMAND_MYSQL_DSN is required for the Agent Message command MySQL integration test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate MySQL: %v", err)
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL Message store: %v", err)
	}
	repository, err := messagemysql.NewMessageRepository(store)
	if err != nil {
		t.Fatalf("create SQLC Message repository: %v", err)
	}
	service := messagedomain.NewMessageService(repository, activeMessageCommandUserFinder{}, nil, nil, nil, nil, nil)
	messageServer, err := NewServer(service)
	if err != nil {
		t.Fatalf("create Message gRPC server: %v", err)
	}
	remote, closeRemote := newCoreMessageClientThroughResponseLoss(t, &postCommitUnavailableMessageServer{delegate: messageServer})
	defer closeRemote()

	commands, err := agentapplication.NewLocalAgentCommandV1(remote)
	if err != nil {
		t.Fatalf("create Agent command service: %v", err)
	}
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions: []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation,
			ResourceID:   model.DirectConversationKey("U100", "UAI"),
			Actions:      []string{application.AgentResourceActionWrite},
		}},
	}
	commandID := fmt.Sprintf("tool:mysql-response-loss:%d", time.Now().UTC().UnixNano())
	result, err := commands.SendMessage(context.Background(), application.AgentMessageCommandV1{
		CommandID: commandID, Kind: application.AgentMessageCommandSystemMessageV1,
		Invocation: invocation, Content: "durable MySQL notice",
	})
	if err != nil {
		t.Fatalf("recover MySQL-backed Message command: %v", err)
	}
	if result == nil || result.UUID == "" || result.ClientMessageID == "" || result.MessageType != model.MessageTypeSystem {
		t.Fatalf("unexpected recovered MySQL Message=%+v", result)
	}
	for _, table := range []string{"messages", "message_metadata"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+mysqlMessageRecoveryPredicate(table), result.UUID).Scan(&count); err != nil {
			t.Fatalf("count %s rows: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s rows=%d, want one committed Message side effect", table, count)
		}
	}
	var inboxRows, inboxRecipients int
	if err := db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT user_uuid) FROM user_sync_inbox WHERE message_uuid = ?", result.UUID).Scan(&inboxRows, &inboxRecipients); err != nil {
		t.Fatalf("count Sync inbox rows: %v", err)
	}
	if inboxRows != 2 || inboxRecipients != 2 {
		t.Fatalf("Sync inbox rows=%d recipients=%d, want sender and target once each", inboxRows, inboxRecipients)
	}
}

func mysqlMessageRecoveryPredicate(table string) string {
	switch table {
	case "messages":
		return "uuid = ?"
	case "message_metadata":
		return "message_uuid = ?"
	default:
		return "1 = 0"
	}
}
