package repository

import (
	"reflect"
	"testing"
)

func TestMessageRepositoryExposesOnlySyncAwareWriteEntrypoints(t *testing.T) {
	t.Parallel()

	repositoryType := reflect.TypeOf((*MessageRepository)(nil))
	for _, method := range []string{"CreateWithSync", "StoreWithOutboxAndSync"} {
		if _, exists := repositoryType.MethodByName(method); !exists {
			t.Fatalf("MessageRepository is missing required write method %s", method)
		}
	}
	for _, method := range []string{"Create", "StoreWithOutbox"} {
		if _, exists := repositoryType.MethodByName(method); exists {
			t.Fatalf("MessageRepository must not expose compatibility write method %s", method)
		}
	}
}
