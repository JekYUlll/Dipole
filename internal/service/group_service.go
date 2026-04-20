package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

var (
	ErrGroupNameRequired             = errors.New("group name is required")
	ErrGroupNameTooLong              = errors.New("group name is too long")
	ErrGroupNoticeTooLong            = errors.New("group notice is too long")
	ErrGroupAvatarTooLong            = errors.New("group avatar is too long")
	ErrGroupEmptyUpdate              = errors.New("group update is empty")
	ErrGroupNotFound                 = errors.New("group not found")
	ErrGroupDismissed                = errors.New("group dismissed")
	ErrGroupPermissionDenied         = errors.New("group permission denied")
	ErrGroupMemberRequired           = errors.New("group member is required")
	ErrGroupMemberUnavailable        = errors.New("group member is unavailable")
	ErrGroupMemberAlreadyIn          = errors.New("group member already exists")
	ErrGroupOwnerCannotLeave         = errors.New("group owner cannot leave")
	ErrGroupOwnerCannotBeRemoved     = errors.New("group owner cannot be removed")
	ErrGroupAvatarMissing            = errors.New("group avatar is missing")
	ErrGroupAvatarInvalid            = errors.New("group avatar is invalid")
	ErrGroupAvatarTooLarge           = errors.New("group avatar is too large")
	ErrGroupAvatarStorageUnavailable = errors.New("group avatar storage is unavailable")
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

type groupAvatarFileRepository interface {
	Create(file *model.UploadedFile) error
	GetByUUID(uuid string) (*model.UploadedFile, error)
}

type groupAvatarStorage interface {
	UploadGroupAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, groupUUID string) (*platformStorage.UploadedObject, error)
	PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error)
}

type GroupAvatarResponse struct {
	RedirectURL string
	ContentType string
	ContentSize int64
	Content     io.ReadCloser
	Cleanup     func()
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
	Group              *model.Group
	Owner              *model.User
	MeRole             int8
	Members            []*GroupMemberView
	IsHot              bool
	RecentMessageCount int
}

type GroupMemberView struct {
	Member *model.GroupMember
	User   *model.User
}

type GroupService struct {
	repo           groupRepository
	userFinder     groupUserFinder
	events         eventPublisher
	hotGroups      groupHeatReader
	fileRepo       groupAvatarFileRepository
	storage        groupAvatarStorage
	avatarMaxBytes int64
	avatarURLTTL   time.Duration
}

type groupHeatReader interface {
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type GroupEventPayload struct {
	GroupUUID      string    `json:"group_uuid"`
	GroupName      string    `json:"group_name,omitempty"`
	Name           string    `json:"name,omitempty"`
	Notice         string    `json:"notice,omitempty"`
	Avatar         string    `json:"avatar,omitempty"`
	OperatorUUID   string    `json:"operator_uuid,omitempty"`
	MemberUUIDs    []string  `json:"member_uuids,omitempty"`
	RecipientUUIDs []string  `json:"recipient_uuids,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func NewGroupService(repo groupRepository, userFinder groupUserFinder, events eventPublisher, hotGroups groupHeatReader) *GroupService {
	return &GroupService{
		repo:           repo,
		userFinder:     userFinder,
		events:         events,
		hotGroups:      hotGroups,
		avatarMaxBytes: 5 * 1024 * 1024,
		avatarURLTTL:   10 * time.Minute,
	}
}

func (s *GroupService) WithAvatarStorage(fileRepo groupAvatarFileRepository, storage groupAvatarStorage, maxBytes int64, urlTTL time.Duration) *GroupService {
	s.fileRepo = fileRepo
	s.storage = storage
	if maxBytes > 0 {
		s.avatarMaxBytes = maxBytes
	}
	if urlTTL > 0 {
		s.avatarURLTTL = urlTTL
	}
	return s
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
	s.publishGroupEvent("group.created", group.UUID, GroupEventPayload{
		GroupUUID:      group.UUID,
		Name:           group.Name,
		Notice:         group.Notice,
		Avatar:         group.Avatar,
		OperatorUUID:   group.OwnerUUID,
		MemberUUIDs:    extractMemberUUIDs(members),
		RecipientUUIDs: extractMemberUUIDs(members),
		OccurredAt:     now,
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
		Group:              group,
		Owner:              owner,
		MeRole:             model.GroupMemberRoleOwner,
		Members:            memberViews,
		IsHot:              false,
		RecentMessageCount: 0,
	}, nil
}

func (s *GroupService) GetGroup(currentUserUUID, groupUUID string) (*GroupView, error) {
	group, currentMember, err := s.loadReadableGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, err
	}

	owner, err := s.userFinder.GetByUUID(group.OwnerUUID)
	if err != nil {
		return nil, fmt.Errorf("get group owner: %w", err)
	}

	members, err := s.repo.ListMembers(strings.TrimSpace(groupUUID))
	if err != nil {
		return nil, fmt.Errorf("list members for get group: %w", err)
	}
	userUUIDs := make([]string, 0, len(members))
	for _, m := range members {
		userUUIDs = append(userUUIDs, m.UserUUID)
	}
	users, err := s.userFinder.ListByUUIDs(userUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list users for get group: %w", err)
	}
	userByUUID := make(map[string]*model.User, len(users))
	for _, u := range users {
		userByUUID[u.UUID] = u
	}
	memberViews := make([]*GroupMemberView, 0, len(members))
	for _, m := range members {
		memberViews = append(memberViews, &GroupMemberView{Member: m, User: userByUUID[m.UserUUID]})
	}

	heatStatus, err := s.groupHeatStatus(group)
	if err != nil {
		return nil, err
	}

	return &GroupView{
		Group:              group,
		Owner:              owner,
		MeRole:             currentMember.Role,
		Members:            memberViews,
		IsHot:              heatStatus.IsHot,
		RecentMessageCount: heatStatus.RecentMessageCount,
	}, nil
}

func (s *GroupService) ListMembers(currentUserUUID, groupUUID string) ([]*GroupMemberView, error) {
	_, _, err := s.loadReadableGroup(currentUserUUID, groupUUID)
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

func (s *GroupService) groupHeatStatus(group *model.Group) (platformHotGroup.Status, error) {
	if group == nil || s.hotGroups == nil {
		return platformHotGroup.Status{}, nil
	}

	status, err := s.hotGroups.Status(group.UUID, group.MemberCount)
	if err != nil {
		return platformHotGroup.Status{}, fmt.Errorf("get group heat status: %w", err)
	}

	return status, nil
}

func (s *GroupService) AddMembers(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*GroupMemberView, error) {
	group, currentMember, err := s.loadWritableGroup(currentUserUUID, groupUUID)
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

	recipients, err := s.listMemberUUIDs(group.UUID)
	if err != nil {
		return nil, fmt.Errorf("list members after add group members: %w", err)
	}
	s.publishGroupEvent("group.members.added", group.UUID, GroupEventPayload{
		GroupUUID:      group.UUID,
		OperatorUUID:   currentUserUUID,
		MemberUUIDs:    extractMemberUUIDs(addedMembers),
		RecipientUUIDs: recipients,
		OccurredAt:     now,
	})

	return views, nil
}

func (s *GroupService) LeaveGroup(currentUserUUID, groupUUID string) error {
	group, currentMember, err := s.loadWritableGroup(currentUserUUID, groupUUID)
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
	group, currentMember, err := s.loadWritableGroup(currentUserUUID, groupUUID)
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
	recipients, err := s.listMemberUUIDs(group.UUID)
	if err != nil {
		return nil, fmt.Errorf("list members after update group: %w", err)
	}
	s.publishGroupEvent("group.updated", group.UUID, GroupEventPayload{
		GroupUUID:      group.UUID,
		Name:           group.Name,
		Notice:         group.Notice,
		Avatar:         group.Avatar,
		OperatorUUID:   currentUserUUID,
		RecipientUUIDs: recipients,
		OccurredAt:     group.UpdatedAt,
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

func (s *GroupService) UploadAvatar(currentUserUUID, groupUUID string, header *multipart.FileHeader) (*GroupView, error) {
	if header == nil {
		return nil, ErrGroupAvatarMissing
	}
	if s.storage == nil || s.fileRepo == nil {
		return nil, ErrGroupAvatarStorageUnavailable
	}
	if s.avatarMaxBytes > 0 && header.Size > s.avatarMaxBytes {
		return nil, ErrGroupAvatarTooLarge
	}
	if !isSupportedGroupAvatarHeader(header) {
		return nil, ErrGroupAvatarInvalid
	}

	group, currentMember, err := s.loadWritableGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, err
	}
	if currentMember.Role != model.GroupMemberRoleOwner {
		return nil, ErrGroupPermissionDenied
	}

	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open group avatar file: %w", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	uploaded, err := s.storage.UploadGroupAvatar(ctx, file, header, group.UUID)
	if err != nil {
		return nil, fmt.Errorf("upload group avatar: %w", err)
	}

	record := &model.UploadedFile{
		UUID:         generateGroupAvatarFileUUID(),
		UploaderUUID: currentUserUUID,
		Bucket:       uploaded.Bucket,
		ObjectKey:    uploaded.ObjectKey,
		FileName:     uploaded.FileName,
		FileSize:     uploaded.FileSize,
		ContentType:  uploaded.ContentType,
		URL:          uploaded.URL,
	}
	if err := s.fileRepo.Create(record); err != nil {
		return nil, fmt.Errorf("persist group avatar file: %w", err)
	}

	group.Avatar = buildGroupAvatarPath(group.UUID)
	group.AvatarFileUUID = record.UUID
	group.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(group); err != nil {
		return nil, fmt.Errorf("update group avatar: %w", err)
	}

	recipients, err := s.listMemberUUIDs(group.UUID)
	if err != nil {
		return nil, fmt.Errorf("list members after update group avatar: %w", err)
	}
	s.publishGroupEvent("group.updated", group.UUID, GroupEventPayload{
		GroupUUID:      group.UUID,
		Name:           group.Name,
		Notice:         group.Notice,
		Avatar:         group.Avatar,
		OperatorUUID:   currentUserUUID,
		RecipientUUIDs: recipients,
		OccurredAt:     group.UpdatedAt,
	})

	owner, err := s.userFinder.GetByUUID(group.OwnerUUID)
	if err != nil {
		return nil, fmt.Errorf("get group owner after avatar update: %w", err)
	}

	return &GroupView{
		Group:  group,
		Owner:  owner,
		MeRole: currentMember.Role,
	}, nil
}

func (s *GroupService) GetAvatarResponse(groupUUID string) (*GroupAvatarResponse, error) {
	group, err := s.repo.GetByUUID(strings.TrimSpace(groupUUID))
	if err != nil {
		return nil, fmt.Errorf("get group for avatar: %w", err)
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	if strings.TrimSpace(group.AvatarFileUUID) == "" {
		if strings.TrimSpace(group.Avatar) == "" {
			return nil, ErrGroupAvatarMissing
		}
		return &GroupAvatarResponse{RedirectURL: group.Avatar}, nil
	}
	if s.storage == nil || s.fileRepo == nil {
		return nil, ErrGroupAvatarStorageUnavailable
	}

	record, err := s.fileRepo.GetByUUID(group.AvatarFileUUID)
	if err != nil {
		return nil, fmt.Errorf("get group avatar file: %w", err)
	}
	if record == nil {
		return nil, ErrGroupAvatarMissing
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	content, err := s.storage.OpenObject(ctx, record.Bucket, record.ObjectKey)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open group avatar object: %w", err)
	}

	return &GroupAvatarResponse{
		ContentType: record.ContentType,
		ContentSize: record.FileSize,
		Content:     content,
		Cleanup: func() {
			cancel()
		},
	}, nil
}

func (s *GroupService) RemoveMembers(currentUserUUID, groupUUID string, memberUUIDs []string) error {
	group, currentMember, err := s.loadWritableGroup(currentUserUUID, groupUUID)
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

	recipients, err := s.listMemberUUIDs(group.UUID)
	if err != nil {
		return fmt.Errorf("list members before remove group members: %w", err)
	}
	if err := s.repo.RemoveMembers(group.UUID, memberUUIDs); err != nil {
		return fmt.Errorf("remove group members: %w", err)
	}
	s.publishGroupEvent("group.members.removed", group.UUID, GroupEventPayload{
		GroupUUID:      group.UUID,
		OperatorUUID:   currentUserUUID,
		MemberUUIDs:    memberUUIDs,
		RecipientUUIDs: recipients,
		OccurredAt:     time.Now().UTC(),
	})

	return nil
}

func (s *GroupService) DismissGroup(currentUserUUID, groupUUID string) error {
	group, currentMember, err := s.loadWritableGroup(currentUserUUID, groupUUID)
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
	s.publishGroupEvent("group.dismissed", group.UUID, GroupEventPayload{
		GroupUUID:      group.UUID,
		GroupName:      group.Name,
		OperatorUUID:   currentUserUUID,
		MemberUUIDs:    extractMemberUUIDs(members),
		RecipientUUIDs: extractMemberUUIDs(members),
		OccurredAt:     group.UpdatedAt,
	})

	return nil
}

func (s *GroupService) publishGroupEvent(topic string, key string, payload any) {
	if s.events == nil {
		return
	}

	_ = s.events.PublishEvent(context.Background(), topic, key, topic, payload, nil)
}

func (s *GroupService) listMemberUUIDs(groupUUID string) ([]string, error) {
	members, err := s.repo.ListMembers(groupUUID)
	if err != nil {
		return nil, err
	}

	return extractMemberUUIDs(members), nil
}

// 软解散后的群仍然允许成员读取资料和历史，因此读路径允许 normal/dismissed 两种状态。
func (s *GroupService) loadReadableGroup(currentUserUUID, groupUUID string) (*model.Group, *model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	group, err := s.repo.GetByUUID(groupUUID)
	if err != nil {
		return nil, nil, fmt.Errorf("get group: %w", err)
	}
	if group == nil {
		return nil, nil, ErrGroupNotFound
	}
	if group.Status != model.GroupStatusNormal && group.Status != model.GroupStatusDismissed {
		return nil, nil, ErrGroupNotFound
	}
	currentMember, err := s.repo.GetMember(groupUUID, strings.TrimSpace(currentUserUUID))
	if err != nil {
		return nil, nil, fmt.Errorf("get current group member: %w", err)
	}
	if currentMember == nil {
		return nil, nil, ErrGroupPermissionDenied
	}
	if strings.TrimSpace(group.OwnerUUID) == strings.TrimSpace(currentUserUUID) {
		currentMember.Role = model.GroupMemberRoleOwner
	}

	return group, currentMember, nil
}

func (s *GroupService) loadWritableGroup(currentUserUUID, groupUUID string) (*model.Group, *model.GroupMember, error) {
	group, currentMember, err := s.loadReadableGroup(currentUserUUID, groupUUID)
	if err != nil {
		return nil, nil, err
	}
	if group.Status == model.GroupStatusDismissed {
		return nil, nil, ErrGroupDismissed
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

func extractMemberUUIDs(members []*model.GroupMember) []string {
	memberUUIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		memberUUIDs = append(memberUUIDs, member.UserUUID)
	}

	return memberUUIDs
}

func generateGroupUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate group uuid: %w", err))
	}

	return "G" + strings.ToUpper(hex.EncodeToString(buf))
}

func generateGroupAvatarFileUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate group avatar file uuid: %w", err))
	}

	return "F" + strings.ToUpper(hex.EncodeToString(buf))
}

func buildGroupAvatarPath(groupUUID string) string {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return ""
	}
	return "/api/v1/groups/" + groupUUID + "/avatar"
}

func isSupportedGroupAvatarHeader(header *multipart.FileHeader) bool {
	if header == nil {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(header.Filename)))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}
