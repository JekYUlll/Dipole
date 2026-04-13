package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrGroupNameRequired         = errors.New("group name is required")
	ErrGroupNameTooLong          = errors.New("group name is too long")
	ErrGroupNoticeTooLong        = errors.New("group notice is too long")
	ErrGroupAvatarTooLong        = errors.New("group avatar is too long")
	ErrGroupEmptyUpdate          = errors.New("group update is empty")
	ErrGroupNotFound             = errors.New("group not found")
	ErrGroupPermissionDenied     = errors.New("group permission denied")
	ErrGroupMemberRequired       = errors.New("group member is required")
	ErrGroupMemberUnavailable    = errors.New("group member is unavailable")
	ErrGroupMemberAlreadyIn      = errors.New("group member already exists")
	ErrGroupOwnerCannotLeave     = errors.New("group owner cannot leave")
	ErrGroupOwnerCannotBeRemoved = errors.New("group owner cannot be removed")
)

type groupRepository interface {
	Create(group *model.Group, members []*model.GroupMember) error
	GetByUUID(groupUUID string) (*model.Group, error)
	GetMember(groupUUID, userUUID string) (*model.GroupMember, error)
	ListMembers(groupUUID string) ([]*model.GroupMember, error)
	AddMembers(groupUUID string, members []*model.GroupMember) error
	Update(group *model.Group) error
	RemoveMembers(groupUUID string, memberUUIDs []string) error
	RemoveMember(groupUUID, userUUID string) error
}

type groupUserFinder interface {
	GetByUUID(uuid string) (*model.User, error)
	ListByUUIDs(uuids []string) ([]*model.User, error)
}

type CreateGroupInput struct {
	Name        string
	Notice      string
	Avatar      string
	MemberUUIDs []string
}

type UpdateGroupInput struct {
	Name   string
	Notice string
	Avatar string
}

type GroupView struct {
	Group   *model.Group
	Owner   *model.User
	MeRole  int8
	Members []*GroupMemberView
}

type GroupMemberView struct {
	Member *model.GroupMember
	User   *model.User
}

type GroupService struct {
	repo       groupRepository
	userFinder groupUserFinder
	notifier   groupNotifier
	events     eventPublisher
}

type groupNotifier interface {
	NotifyGroupDismissed(groupUUID, groupName string, memberUUIDs []string)
}

func NewGroupService(repo groupRepository, userFinder groupUserFinder, notifier groupNotifier, events eventPublisher) *GroupService {
	return &GroupService{
		repo:       repo,
		userFinder: userFinder,
		notifier:   notifier,
		events:     events,
	}
}

func (s *GroupService) CreateGroup(currentUserUUID string, input CreateGroupInput) (*GroupView, error) {
	name := strings.TrimSpace(input.Name)
	notice := strings.TrimSpace(input.Notice)
	avatar := strings.TrimSpace(input.Avatar)
	switch {
	case name == "":
		return nil, ErrGroupNameRequired
	case len([]rune(name)) > 50:
		return nil, ErrGroupNameTooLong
	case len([]rune(notice)) > 500:
		return nil, ErrGroupNoticeTooLong
	case len([]rune(avatar)) > 255:
		return nil, ErrGroupAvatarTooLong
	}

	memberUUIDs := normalizeGroupMemberUUIDs(input.MemberUUIDs, currentUserUUID)
	memberUsers, err := s.loadAvailableUsers(memberUUIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	group := &model.Group{
		UUID:        generateGroupUUID(),
		Name:        name,
		Notice:      notice,
		Avatar:      avatar,
		OwnerUUID:   currentUserUUID,
		MemberCount: len(memberUUIDs) + 1,
		Status:      model.GroupStatusNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if group.Avatar == "" {
		group.Avatar = model.DefaultAvatarURL
	}

	members := make([]*model.GroupMember, 0, len(memberUUIDs)+1)
	ownerMember := &model.GroupMember{
		GroupUUID: group.UUID,
		UserUUID:  currentUserUUID,
		Role:      model.GroupMemberRoleOwner,
		JoinedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	members = append(members, ownerMember)
	for _, memberUUID := range memberUUIDs {
		members = append(members, &model.GroupMember{
			GroupUUID: group.UUID,
			UserUUID:  memberUUID,
			Role:      model.GroupMemberRoleMember,
			JoinedAt:  now,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	if err := s.repo.Create(group, members); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	s.publishGroupEvent("group.created", group.UUID, map[string]any{
		"group_uuid":   group.UUID,
		"name":         group.Name,
		"owner_uuid":   group.OwnerUUID,
		"member_count": group.MemberCount,
	})

	owner, err := s.userFinder.GetByUUID(currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("get group owner after create: %w", err)
	}

	memberViews := make([]*GroupMemberView, 0, len(members))
	memberUserByUUID := map[string]*model.User{currentUserUUID: owner}
	for _, user := range memberUsers {
		memberUserByUUID[user.UUID] = user
	}
	for _, member := range members {
		memberViews = append(memberViews, &GroupMemberView{
			Member: member,
			User:   memberUserByUUID[member.UserUUID],
		})
	}

	return &GroupView{
		Group:   group,
		Owner:   owner,
		MeRole:  model.GroupMemberRoleOwner,
		Members: memberViews,
	}, nil
}

func (s *GroupService) GetGroup(currentUserUUID, groupUUID string) (*GroupView, error) {
	group, currentMember, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, err
	}

	owner, err := s.userFinder.GetByUUID(group.OwnerUUID)
	if err != nil {
		return nil, fmt.Errorf("get group owner: %w", err)
	}

	return &GroupView{
		Group:  group,
		Owner:  owner,
		MeRole: currentMember.Role,
	}, nil
}

func (s *GroupService) ListMembers(currentUserUUID, groupUUID string) ([]*GroupMemberView, error) {
	_, _, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.ListMembers(strings.TrimSpace(groupUUID))
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	userUUIDs := make([]string, 0, len(members))
	for _, member := range members {
		userUUIDs = append(userUUIDs, member.UserUUID)
	}
	users, err := s.userFinder.ListByUUIDs(userUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list users for group members: %w", err)
	}
	userByUUID := make(map[string]*model.User, len(users))
	for _, user := range users {
		userByUUID[user.UUID] = user
	}

	views := make([]*GroupMemberView, 0, len(members))
	for _, member := range members {
		views = append(views, &GroupMemberView{
			Member: member,
			User:   userByUUID[member.UserUUID],
		})
	}

	return views, nil
}

func (s *GroupService) AddMembers(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*GroupMemberView, error) {
	group, currentMember, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, err
	}
	if currentMember.Role != model.GroupMemberRoleOwner {
		return nil, ErrGroupPermissionDenied
	}

	memberUUIDs = normalizeGroupMemberUUIDs(memberUUIDs, "")
	if len(memberUUIDs) == 0 {
		return nil, ErrGroupMemberRequired
	}

	users, err := s.loadAvailableUsers(memberUUIDs)
	if err != nil {
		return nil, err
	}

	addedMembers := make([]*model.GroupMember, 0, len(memberUUIDs))
	now := time.Now().UTC()
	for _, memberUUID := range memberUUIDs {
		existing, err := s.repo.GetMember(group.UUID, memberUUID)
		if err != nil {
			return nil, fmt.Errorf("check existing group member: %w", err)
		}
		if existing != nil {
			return nil, ErrGroupMemberAlreadyIn
		}
		addedMembers = append(addedMembers, &model.GroupMember{
			GroupUUID: group.UUID,
			UserUUID:  memberUUID,
			Role:      model.GroupMemberRoleMember,
			JoinedAt:  now,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	if err := s.repo.AddMembers(group.UUID, addedMembers); err != nil {
		return nil, fmt.Errorf("add group members: %w", err)
	}

	group.MemberCount += len(addedMembers)
	userByUUID := make(map[string]*model.User, len(users))
	for _, user := range users {
		userByUUID[user.UUID] = user
	}

	views := make([]*GroupMemberView, 0, len(addedMembers))
	for _, member := range addedMembers {
		views = append(views, &GroupMemberView{
			Member: member,
			User:   userByUUID[member.UserUUID],
		})
	}

	return views, nil
}

func (s *GroupService) LeaveGroup(currentUserUUID, groupUUID string) error {
	group, currentMember, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return err
	}
	if currentMember.Role == model.GroupMemberRoleOwner || group.OwnerUUID == currentUserUUID {
		return ErrGroupOwnerCannotLeave
	}
	if err := s.repo.RemoveMember(group.UUID, currentUserUUID); err != nil {
		return fmt.Errorf("leave group: %w", err)
	}

	return nil
}

func (s *GroupService) UpdateGroup(currentUserUUID, groupUUID string, input UpdateGroupInput) (*GroupView, error) {
	group, currentMember, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, err
	}
	if currentMember.Role != model.GroupMemberRoleOwner {
		return nil, ErrGroupPermissionDenied
	}

	name := strings.TrimSpace(input.Name)
	notice := strings.TrimSpace(input.Notice)
	avatar := strings.TrimSpace(input.Avatar)
	if name == "" && notice == "" && avatar == "" {
		return nil, ErrGroupEmptyUpdate
	}
	if name != "" {
		if len([]rune(name)) > 50 {
			return nil, ErrGroupNameTooLong
		}
		group.Name = name
	}
	if notice != "" {
		if len([]rune(notice)) > 500 {
			return nil, ErrGroupNoticeTooLong
		}
		group.Notice = notice
	}
	if avatar != "" {
		if len([]rune(avatar)) > 255 {
			return nil, ErrGroupAvatarTooLong
		}
		group.Avatar = avatar
	}
	group.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(group); err != nil {
		return nil, fmt.Errorf("update group: %w", err)
	}
	s.publishGroupEvent("group.updated", group.UUID, map[string]any{
		"group_uuid": group.UUID,
		"name":       group.Name,
		"notice":     group.Notice,
		"avatar":     group.Avatar,
	})

	owner, err := s.userFinder.GetByUUID(group.OwnerUUID)
	if err != nil {
		return nil, fmt.Errorf("get group owner after update: %w", err)
	}

	return &GroupView{
		Group:  group,
		Owner:  owner,
		MeRole: currentMember.Role,
	}, nil
}

func (s *GroupService) RemoveMembers(currentUserUUID, groupUUID string, memberUUIDs []string) error {
	group, currentMember, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return err
	}
	if currentMember.Role != model.GroupMemberRoleOwner {
		return ErrGroupPermissionDenied
	}

	memberUUIDs = normalizeGroupMemberUUIDs(memberUUIDs, "")
	if len(memberUUIDs) == 0 {
		return ErrGroupMemberRequired
	}
	for _, memberUUID := range memberUUIDs {
		if memberUUID == group.OwnerUUID {
			return ErrGroupOwnerCannotBeRemoved
		}
		member, err := s.repo.GetMember(group.UUID, memberUUID)
		if err != nil {
			return fmt.Errorf("check member before remove: %w", err)
		}
		if member == nil {
			return ErrGroupMemberUnavailable
		}
	}

	if err := s.repo.RemoveMembers(group.UUID, memberUUIDs); err != nil {
		return fmt.Errorf("remove group members: %w", err)
	}
	s.publishGroupEvent("group.members.removed", group.UUID, map[string]any{
		"group_uuid":   group.UUID,
		"member_uuids": memberUUIDs,
	})

	return nil
}

func (s *GroupService) DismissGroup(currentUserUUID, groupUUID string) error {
	group, currentMember, err := s.loadAccessibleGroup(currentUserUUID, groupUUID)
	if err != nil {
		return err
	}
	if currentMember.Role != model.GroupMemberRoleOwner {
		return ErrGroupPermissionDenied
	}

	members, err := s.repo.ListMembers(group.UUID)
	if err != nil {
		return fmt.Errorf("list members before dismiss group: %w", err)
	}

	group.Status = model.GroupStatusDismissed
	group.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(group); err != nil {
		return fmt.Errorf("dismiss group: %w", err)
	}
	s.publishGroupEvent("group.dismissed", group.UUID, map[string]any{
		"group_uuid": group.UUID,
		"group_name": group.Name,
	})

	if s.notifier != nil {
		memberUUIDs := make([]string, 0, len(members))
		for _, member := range members {
			if member == nil {
				continue
			}
			memberUUIDs = append(memberUUIDs, member.UserUUID)
		}
		s.notifier.NotifyGroupDismissed(group.UUID, group.Name, memberUUIDs)
	}

	return nil
}

func (s *GroupService) publishGroupEvent(topic string, key string, payload any) {
	if s.events == nil {
		return
	}

	_ = s.events.PublishEvent(context.Background(), topic, key, topic, payload, nil)
}

func (s *GroupService) loadAccessibleGroup(currentUserUUID, groupUUID string) (*model.Group, *model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	group, err := s.repo.GetByUUID(groupUUID)
	if err != nil {
		return nil, nil, fmt.Errorf("get group: %w", err)
	}
	if group == nil || group.Status != model.GroupStatusNormal {
		return nil, nil, ErrGroupNotFound
	}
	currentMember, err := s.repo.GetMember(groupUUID, strings.TrimSpace(currentUserUUID))
	if err != nil {
		return nil, nil, fmt.Errorf("get current group member: %w", err)
	}
	if currentMember == nil {
		return nil, nil, ErrGroupPermissionDenied
	}

	return group, currentMember, nil
}

func (s *GroupService) loadAvailableUsers(userUUIDs []string) ([]*model.User, error) {
	if len(userUUIDs) == 0 {
		return nil, nil
	}

	users, err := s.userFinder.ListByUUIDs(userUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list group target users: %w", err)
	}
	if len(users) != len(userUUIDs) {
		return nil, ErrGroupMemberUnavailable
	}
	userByUUID := make(map[string]*model.User, len(users))
	for _, user := range users {
		userByUUID[user.UUID] = user
	}
	for _, userUUID := range userUUIDs {
		user := userByUUID[userUUID]
		if user == nil || user.Status != model.UserStatusNormal {
			return nil, ErrGroupMemberUnavailable
		}
	}

	return users, nil
}

func normalizeGroupMemberUUIDs(userUUIDs []string, skipUUID string) []string {
	seen := make(map[string]struct{}, len(userUUIDs))
	normalized := make([]string, 0, len(userUUIDs))
	for _, userUUID := range userUUIDs {
		userUUID = strings.TrimSpace(userUUID)
		if userUUID == "" || userUUID == skipUUID {
			continue
		}
		if _, ok := seen[userUUID]; ok {
			continue
		}
		seen[userUUID] = struct{}{}
		normalized = append(normalized, userUUID)
	}

	return normalized
}

func generateGroupUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate group uuid: %w", err))
	}

	return "G" + strings.ToUpper(hex.EncodeToString(buf))
}
