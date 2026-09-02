package messagedomain

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	cassandrabackfill "github.com/JekYUlll/Dipole/internal/operations/cassandra/backfill"
	cassandradata "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMessageDuplicateHydrationWithRealCassandra(t *testing.T) {
	hosts := strings.Split(strings.TrimSpace(os.Getenv("DIPOLE_TEST_CASSANDRA_HOSTS")), ",")
	if len(hosts) == 0 || hosts[0] == "" {
		t.Skip("DIPOLE_TEST_CASSANDRA_HOSTS is required")
	}
	session, err := cassandradata.OpenSession(config.Cassandra{
		Enabled: true, Hosts: hosts, Keyspace: "dipole_message_shadow", LocalDatacenter: "datacenter1",
		TimelineBucketSize: 1000, ConnectTimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	timeline, err := cassandradata.NewTimelineStore(session, 1000)
	if err != nil {
		t.Fatal(err)
	}
	suffix := time.Now().UTC().Format("150405.000000")
	existing := &model.Message{
		ID: 42, UUID: "M-dup-" + suffix, ClientMessageID: "C-dup-" + suffix,
		ConversationKey: "direct:U1:U2:" + suffix, Seq: 7,
		SenderUUID: "U1", TargetUUID: "U2", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "from Cassandra", SentAt: time.Now().UTC(),
	}
	if _, err := timeline.Append(t.Context(), cassandrabackfill.ProjectionForMessage(*existing)); err != nil {
		t.Fatal(err)
	}
	hydrator, err := cassandradata.NewSyncMessageHydrator(timeline)
	if err != nil {
		t.Fatal(err)
	}
	repo := &stubMessageRepository{storeWithOutboxErr: &mysqlDriver.MySQLError{Number: 1062}, messagesByUUID: map[string]*model.Message{existing.UUID: existing}}
	messageService := NewMessageService(repo, &stubMessageUserFinder{}, nil, nil, nil, &stubEventPublisher{}, nil)
	messageService.SetDuplicateMessageHydrator(hydrator)
	message, err := messageService.PersistRequestedMessage(MessageEventPayload{
		MessageID: "M-new", ClientMessageID: existing.ClientMessageID, ConversationKey: existing.ConversationKey,
		MessageSeq: existing.Seq, SenderUUID: existing.SenderUUID, TargetUUID: existing.TargetUUID,
		TargetType: existing.TargetType, MessageType: existing.MessageType, Content: existing.Content, SentAt: existing.SentAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if message == nil || message.ID != existing.ID || message.UUID != existing.UUID || message.Content != existing.Content {
		t.Fatalf("unexpected duplicate response: %+v", message)
	}
	if repo.getByUUIDCalls != 0 {
		t.Fatalf("Cassandra hit read MySQL body %d times", repo.getByUUIDCalls)
	}
}
