package application

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type searchCoreStub struct {
	principal string
	keys      []string
	err       error
}

func (s *searchCoreStub) ListSearchConversationKeys(principal string) ([]string, error) {
	s.principal = principal
	return s.keys, s.err
}

func (*searchCoreStub) GetUserByUUID(string) (*model.User, error)                 { return nil, nil }
func (*searchCoreStub) CanSendDirectMessage(string, string) (bool, error)         { return false, nil }
func (*searchCoreStub) GetGroupByUUID(string) (*model.Group, error)               { return nil, nil }
func (*searchCoreStub) GetGroupMember(string, string) (*model.GroupMember, error) { return nil, nil }
func (*searchCoreStub) ListGroupMembers(string) ([]*model.GroupMember, error)     { return nil, nil }
func (*searchCoreStub) GetOwnedFile(string, string) (*model.UploadedFile, error)  { return nil, nil }
func (*searchCoreStub) ListOwnedFiles(string, string, int) (*application.OwnedFilePage, error) {
	return &application.OwnedFilePage{}, nil
}

type searchIndexStub struct {
	query model.MessageSearchQuery
	items []*model.MessageSearchDocument
	err   error
	calls int
}

func (*searchIndexStub) Apply(*model.MessageSearchMutation) error { return nil }

func (s *searchIndexStub) Search(query model.MessageSearchQuery) ([]*model.MessageSearchDocument, error) {
	s.calls++
	s.query = query
	return s.items, s.err
}

func TestSearchApplicationUsesPrincipalDerivedScope(t *testing.T) {
	core := &searchCoreStub{keys: []string{"group:G1", "direct:U1:U2"}}
	want := []*model.MessageSearchDocument{{MessageUUID: "M1", SentAt: time.Unix(1, 0)}}
	index := &searchIndexStub{items: want}
	search, err := NewSearchApplication(core, index)
	if err != nil {
		t.Fatalf("new Search application: %v", err)
	}

	got, err := search.Search("U1", "  migration  ", 25)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Search result: got=%+v want=%+v err=%v", got, want, err)
	}
	if core.principal != "U1" || index.calls != 1 {
		t.Fatalf("authorization path: principal=%q calls=%d", core.principal, index.calls)
	}
	wantQuery := model.MessageSearchQuery{ConversationKeys: core.keys, Text: "migration", Limit: 25}
	if !reflect.DeepEqual(index.query, wantQuery) {
		t.Fatalf("Search query: got=%+v want=%+v", index.query, wantQuery)
	}
}

func TestSearchApplicationFailsClosedOnEmptyScope(t *testing.T) {
	index := &searchIndexStub{}
	search, err := NewSearchApplication(&searchCoreStub{}, index)
	if err != nil {
		t.Fatalf("new Search application: %v", err)
	}

	items, err := search.Search("U1", "migration", 20)
	if err != nil || len(items) != 0 || index.calls != 0 {
		t.Fatalf("empty scope: items=%v calls=%d err=%v", items, index.calls, err)
	}
}

func TestSearchApplicationValidatesInputAndDependencies(t *testing.T) {
	if _, err := NewSearchApplication(nil, &searchIndexStub{}); err == nil {
		t.Fatal("expected missing Core capability to fail")
	}
	if _, err := NewSearchApplication(&searchCoreStub{}, nil); err == nil {
		t.Fatal("expected missing Search index to fail")
	}
	search, err := NewSearchApplication(&searchCoreStub{}, &searchIndexStub{})
	if err != nil {
		t.Fatalf("new Search application: %v", err)
	}
	if _, err := search.Search("U1", "  ", 20); !errors.Is(err, application.ErrSearchTextRequired) {
		t.Fatalf("expected required text error, got %v", err)
	}
}
