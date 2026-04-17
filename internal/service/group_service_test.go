package service

import (
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubGroupRepository struct {
	groups  map[string]*model.Group
	members map[string]map[string]*model.GroupMember
}

func newStubGroupRepository() *stubGroupRepository {
	return &stubGroupRepository{
		groups:  map[string]*model.Group{},
		members: map[string]map[string]*model.GroupMember{},
	}
}

func (r *stubGroupRepository) Create(group *model.Group, members []*model.GroupMember) error {
	r.groups[group.UUID] = group
	if r.members[group.UUID] == nil {
		r.members[group.UUID] = map[string]*model.GroupMember{}
	}
	for _, member := range members {
		r.members[group.UUID][member.UserUUID] = member
	}
	return nil
}

func (r *stubGroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	return r.groups[groupUUID], nil
}

func (r *stubGroupRepository) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return r.members[groupUUID][userUUID], nil
}

func (r *stubGroupRepository) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	members := make([]*model.GroupMember, 0, len(r.members[groupUUID]))
	for _, member := range r.members[groupUUID] {
		members = append(members, member)
	}
	return members, nil
}

func (r *stubGroupRepository) AddMembers(groupUUID string, members []*model.GroupMember) error {
	if r.members[groupUUID] == nil {
		r.members[groupUUID] = map[string]*model.GroupMember{}
	}
	if r.groups[groupUUID] != nil {
		r.groups[groupUUID].MemberCount += len(members)
	}
	for _, member := range members {
		r.members[groupUUID][member.UserUUID] = member
	}
	return nil
}

func (r *stubGroupRepository) Update(group *model.Group) error {
	r.groups[group.UUID] = group
	return nil
}

func (r *stubGroupRepository) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	for _, memberUUID := range memberUUIDs {
		delete(r.members[groupUUID], memberUUID)
		if r.groups[groupUUID] != nil && r.groups[groupUUID].MemberCount > 0 {
			r.groups[groupUUID].MemberCount--
		}
	}
	return nil
}

func (r *stubGroupRepository) RemoveMember(groupUUID, userUUID string) error {
	delete(r.members[groupUUID], userUUID)
	if r.groups[groupUUID] != nil && r.groups[groupUUID].MemberCount > 0 {
		r.groups[groupUUID].MemberCount--
	}
	return nil
}

type stubGroupUserFinder struct {
	users map[string]*model.User
	err   error
}

func (f *stubGroupUserFinder) GetByUUID(uuid string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.users[uuid], nil
}

func (f *stubGroupUserFinder) ListByUUIDs(uuids []string) ([]*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	users := make([]*model.User, 0, len(uuids))
	for _, uuid := range uuids {
		if user := f.users[uuid]; user != nil {
			users = append(users, user)
		}
	}
	return users, nil
}

func TestGroupServiceCreateGroupSuccess(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "owner", Status: model.UserStatusNormal},
			"U200": {UUID: "U200", Nickname: "member", Status: model.UserStatusNormal},
		},
	}, nil)

	group, err := svc.CreateGroup("U100", CreateGroupInput{
		Name:        "Study Group",
		MemberUUIDs: []string{"U200"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if group.Group == nil || group.Group.UUID == "" {
		t.Fatalf("expected created group uuid")
	}
	if group.Group.MemberCount != 2 {
		t.Fatalf("expected member count 2, got %d", group.Group.MemberCount)
	}
	if group.MeRole != model.GroupMemberRoleOwner {
		t.Fatalf("expected owner role, got %d", group.MeRole)
	}
}

func TestGroupServiceCreateGroupRejectsInvalidMember(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Status: model.UserStatusNormal},
		},
	}, nil)

	_, err := svc.CreateGroup("U100", CreateGroupInput{
		Name:        "Study Group",
		MemberUUIDs: []string{"U200"},
	})
	if !errors.Is(err, ErrGroupMemberUnavailable) {
		t.Fatalf("expected ErrGroupMemberUnavailable, got %v", err)
	}
}

func TestGroupServiceGetGroupRequiresMembership(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", OwnerUUID: "U100", Status: model.GroupStatusNormal}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner, JoinedAt: time.Now().UTC()},
	}
	svc := NewGroupService(repo, &stubGroupUserFinder{}, nil)

	_, err := svc.GetGroup("U200", "G100")
	if !errors.Is(err, ErrGroupPermissionDenied) {
		t.Fatalf("expected ErrGroupPermissionDenied, got %v", err)
	}
}

func TestGroupServiceAddMembersRequiresOwner(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", OwnerUUID: "U100", Status: model.GroupStatusNormal, MemberCount: 2}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner},
		"U200": {GroupUUID: "G100", UserUUID: "U200", Role: model.GroupMemberRoleMember},
	}
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U300": {UUID: "U300", Status: model.UserStatusNormal},
		},
	}, nil)

	_, err := svc.AddMembers("U200", "G100", []string{"U300"})
	if !errors.Is(err, ErrGroupPermissionDenied) {
		t.Fatalf("expected ErrGroupPermissionDenied, got %v", err)
	}
}

func TestGroupServiceAddMembersAllowsOwnerWhenStoredRoleIsWrong(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", OwnerUUID: "U100", Status: model.GroupStatusNormal, MemberCount: 2}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleMember},
		"U200": {GroupUUID: "G100", UserUUID: "U200", Role: model.GroupMemberRoleMember},
	}
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U300": {UUID: "U300", Status: model.UserStatusNormal},
		},
	}, nil)

	added, err := svc.AddMembers("U100", "G100", []string{"U300"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(added) != 1 || added[0].Member.UserUUID != "U300" {
		t.Fatalf("expected U300 to be added, got %+v", added)
	}
}

func TestGroupServiceLeaveGroupRejectsOwner(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", OwnerUUID: "U100", Status: model.GroupStatusNormal, MemberCount: 1}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner},
	}
	svc := NewGroupService(repo, &stubGroupUserFinder{}, nil)

	err := svc.LeaveGroup("U100", "G100")
	if !errors.Is(err, ErrGroupOwnerCannotLeave) {
		t.Fatalf("expected ErrGroupOwnerCannotLeave, got %v", err)
	}
}

func TestGroupServiceUpdateGroupSuccess(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", Name: "Old", OwnerUUID: "U100", Status: model.GroupStatusNormal}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner},
	}
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "owner"},
		},
	}, nil)

	view, err := svc.UpdateGroup("U100", "G100", UpdateGroupInput{Name: "New Name", Notice: "hello"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if view.Group.Name != "New Name" {
		t.Fatalf("expected updated name, got %s", view.Group.Name)
	}
}

func TestGroupServiceRemoveMembersRejectsOwner(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", Name: "Team", OwnerUUID: "U100", Status: model.GroupStatusNormal, MemberCount: 2}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner},
		"U200": {GroupUUID: "G100", UserUUID: "U200", Role: model.GroupMemberRoleMember},
	}
	svc := NewGroupService(repo, &stubGroupUserFinder{}, nil)

	err := svc.RemoveMembers("U100", "G100", []string{"U100"})
	if !errors.Is(err, ErrGroupOwnerCannotBeRemoved) {
		t.Fatalf("expected ErrGroupOwnerCannotBeRemoved, got %v", err)
	}
}

func TestGroupServiceDismissGroupPublishesEvent(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	repo.groups["G100"] = &model.Group{UUID: "G100", Name: "Team", OwnerUUID: "U100", Status: model.GroupStatusNormal, MemberCount: 2}
	repo.members["G100"] = map[string]*model.GroupMember{
		"U100": {GroupUUID: "G100", UserUUID: "U100", Role: model.GroupMemberRoleOwner},
		"U200": {GroupUUID: "G100", UserUUID: "U200", Role: model.GroupMemberRoleMember},
	}
	publisher := &stubEventPublisher{}
	svc := NewGroupService(repo, &stubGroupUserFinder{}, publisher)

	err := svc.DismissGroup("U100", "G100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.groups["G100"].Status != model.GroupStatusDismissed {
		t.Fatalf("expected dismissed group status, got %d", repo.groups["G100"].Status)
	}
	if len(publisher.topics) != 1 || publisher.topics[0] != "group.dismissed" {
		t.Fatalf("expected group.dismissed event, got %+v", publisher.topics)
	}
}

func TestGroupServicePublishesKafkaEventOnCreate(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	publisher := &stubEventPublisher{}
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "owner", Status: model.UserStatusNormal},
		},
	}, publisher)

	if _, err := svc.CreateGroup("U100", CreateGroupInput{Name: "Kafka Group"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(publisher.topics) != 1 || publisher.topics[0] != "group.created" {
		t.Fatalf("expected group.created event, got %+v", publisher.topics)
	}
}

func TestGroupServiceCreatePublishesRecipientsForInitialMembers(t *testing.T) {
	t.Parallel()

	repo := newStubGroupRepository()
	publisher := &stubEventPublisher{}
	svc := NewGroupService(repo, &stubGroupUserFinder{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "owner", Status: model.UserStatusNormal},
			"U200": {UUID: "U200", Nickname: "member", Status: model.UserStatusNormal},
		},
	}, publisher)

	if _, err := svc.CreateGroup("U100", CreateGroupInput{Name: "Invite Group", MemberUUIDs: []string{"U200"}}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	payload, ok := publisher.payloads[0].(GroupEventPayload)
	if !ok {
		t.Fatalf("expected GroupEventPayload, got %T", publisher.payloads[0])
	}
	if len(payload.RecipientUUIDs) != 2 {
		t.Fatalf("expected 2 recipients, got %+v", payload.RecipientUUIDs)
	}
	if len(payload.MemberUUIDs) != 2 {
		t.Fatalf("expected 2 member uuids, got %+v", payload.MemberUUIDs)
	}
}
