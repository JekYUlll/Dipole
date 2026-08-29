package coremysql

import (
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
)

type groupStoreSpy struct {
	groups      map[string]*model.Group
	members     map[string]map[string]*model.GroupMember
	groupReads  int
	memberLists int
}

func (s *groupStoreSpy) Create(group *model.Group, members []*model.GroupMember) error {
	s.groups[group.UUID] = cloneGroup(group)
	if s.members[group.UUID] == nil {
		s.members[group.UUID] = map[string]*model.GroupMember{}
	}
	for _, member := range members {
		if member != nil {
			s.members[group.UUID][member.UserUUID] = cloneGroupMember(member)
		}
	}
	return nil
}

func (s *groupStoreSpy) GetByUUID(groupUUID string) (*model.Group, error) {
	s.groupReads++
	return cloneGroup(s.groups[groupUUID]), nil
}

func (s *groupStoreSpy) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return cloneGroupMember(s.members[groupUUID][userUUID]), nil
}

func (s *groupStoreSpy) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	s.memberLists++
	members := make([]*model.GroupMember, 0, len(s.members[groupUUID]))
	for _, member := range s.members[groupUUID] {
		members = append(members, cloneGroupMember(member))
	}
	return members, nil
}

func (s *groupStoreSpy) AddMembers(groupUUID string, members []*model.GroupMember) error {
	if s.members[groupUUID] == nil {
		s.members[groupUUID] = map[string]*model.GroupMember{}
	}
	for _, member := range members {
		if member == nil {
			continue
		}
		if _, exists := s.members[groupUUID][member.UserUUID]; !exists {
			s.members[groupUUID][member.UserUUID] = cloneGroupMember(member)
			s.groups[groupUUID].MemberCount++
		}
	}
	return nil
}

func (s *groupStoreSpy) Update(group *model.Group) error {
	s.groups[group.UUID] = cloneGroup(group)
	return nil
}

func (s *groupStoreSpy) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	for _, userUUID := range memberUUIDs {
		if _, exists := s.members[groupUUID][userUUID]; exists {
			delete(s.members[groupUUID], userUUID)
			s.groups[groupUUID].MemberCount--
		}
	}
	return nil
}

func (s *groupStoreSpy) RemoveMember(groupUUID, userUUID string) error {
	return s.RemoveMembers(groupUUID, []string{userUUID})
}

func TestCachedGroupStoreCachesMetadataAndRefreshesUpdate(t *testing.T) {
	cleanup := setupCachedUserStoreTest(t)
	defer cleanup()

	backend := newGroupStoreSpy()
	backend.groups["G100"] = &model.Group{UUID: "G100", Name: "old", Status: model.GroupStatusNormal}
	platformBloom.Load(nil, []string{"G100"})
	groups := NewCachedGroupStore(backend)

	first, err := groups.GetByUUID("G100")
	if err != nil || first == nil || first.Name != "old" {
		t.Fatalf("first group read: group=%+v err=%v", first, err)
	}
	backend.groups["G100"].Name = "database-change"
	second, err := groups.GetByUUID("G100")
	if err != nil || second == nil || second.Name != "old" || backend.groupReads != 1 {
		t.Fatalf("cached group read: group=%+v calls=%d err=%v", second, backend.groupReads, err)
	}
	second.Name = "new"
	if err := groups.Update(second); err != nil {
		t.Fatalf("update group: %v", err)
	}
	updated, err := groups.GetByUUID("G100")
	if err != nil || updated == nil || updated.Name != "new" || backend.groupReads != 1 {
		t.Fatalf("updated group read: group=%+v calls=%d err=%v", updated, backend.groupReads, err)
	}

	missing, err := groups.GetByUUID("G404")
	if err != nil || missing != nil || backend.groupReads != 1 {
		t.Fatalf("bloom rejection: group=%+v calls=%d err=%v", missing, backend.groupReads, err)
	}
}

func TestCachedGroupStoreCachesSortedMembersAndInvalidatesMutations(t *testing.T) {
	cleanup := setupCachedUserStoreTest(t)
	defer cleanup()

	now := time.Now().UTC()
	backend := newGroupStoreSpy()
	backend.groups["G100"] = &model.Group{UUID: "G100", MemberCount: 2}
	backend.members["G100"] = map[string]*model.GroupMember{
		"U200": {GroupUUID: "G100", UserUUID: "U200", Role: model.GroupMemberRoleMember, JoinedAt: now.Add(time.Second)},
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner, JoinedAt: now},
	}
	platformBloom.Load(nil, []string{"G100"})
	groups := NewCachedGroupStore(backend)

	members, err := groups.ListMembers("G100")
	if err != nil || len(members) != 2 || members[0].UserUUID != "U100" {
		t.Fatalf("first member list: members=%+v err=%v", members, err)
	}
	members, err = groups.ListMembers("G100")
	if err != nil || len(members) != 2 || backend.memberLists != 1 {
		t.Fatalf("cached member list: members=%+v calls=%d err=%v", members, backend.memberLists, err)
	}
	newMember := &model.GroupMember{GroupUUID: "G100", UserUUID: "U300", Role: model.GroupMemberRoleMember, JoinedAt: now.Add(2 * time.Second)}
	if err := groups.AddMembers("G100", []*model.GroupMember{newMember}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	members, err = groups.ListMembers("G100")
	if err != nil || len(members) != 3 || backend.memberLists != 2 {
		t.Fatalf("member list after invalidation: members=%+v calls=%d err=%v", members, backend.memberLists, err)
	}
}

func newGroupStoreSpy() *groupStoreSpy {
	return &groupStoreSpy{
		groups:  map[string]*model.Group{},
		members: map[string]map[string]*model.GroupMember{},
	}
}

func cloneGroup(group *model.Group) *model.Group {
	if group == nil {
		return nil
	}
	copy := *group
	return &copy
}

func cloneGroupMember(member *model.GroupMember) *model.GroupMember {
	if member == nil {
		return nil
	}
	copy := *member
	return &copy
}
