package syncmysql

import (
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestValidateHydratedSyncMessagesRejectsMissingAndLocatorConflict(t *testing.T) {
	locator := model.SyncMessageLocator{MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 7}
	if err := validateHydratedSyncMessages([]model.SyncMessageLocator{locator}, nil); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing error=%v", err)
	}
	messages := map[string]*model.Message{"M1": {UUID: "M1", ConversationKey: "group:G1", Seq: 8}}
	if err := validateHydratedSyncMessages([]model.SyncMessageLocator{locator}, messages); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict error=%v", err)
	}
}
