package bootstrap

import (
	"github.com/JekYUlll/Dipole/internal/application"
	messagekafka "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/kafka"
)

// The embedded runtime keeps this narrow wrapper as its rollback entry point.
type outboxRelay = messagekafka.Relay

func newOutboxRelay(repo application.OutboxRelayStore) *outboxRelay {
	return messagekafka.NewRelay(repo)
}
