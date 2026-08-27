package app

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
)

var _ application.GroupStore = (*CachedGroupStore)(nil)

type CachedGroupStore struct {
	backend application.GroupStore
}

func NewCachedGroupStore(backend application.GroupStore) *CachedGroupStore {
	if backend == nil {
		panic("cached group store backend is required")
	}
	return &CachedGroupStore{backend: backend}
}

func (s *CachedGroupStore) Create(group *model.Group, members []*model.GroupMember) error {
	if err := s.backend.Create(group, members); err != nil {
		return err
	}
	if group == nil {
		return nil
	}
	platformBloom.AddGroup(group.UUID)
	s.cacheGroup(group)
	s.cacheMembers(group.UUID, members)
	return nil
}

func (s *CachedGroupStore) GetByUUID(groupUUID string) (*model.Group, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" || !platformBloom.GroupMayExist(groupUUID) {
		return nil, nil
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	var cached model.Group
	if hit, err := platformCache.GetJSON(ctx, platformCache.GroupMetaKey(groupUUID), &cached); err == nil && hit {
		return &cached, nil
	}
	group, err := s.backend.GetByUUID(groupUUID)
	if err != nil || group == nil {
		return group, err
	}
	_ = platformCache.SetJSON(ctx, platformCache.GroupMetaKey(groupUUID), group, platformCache.GroupMetaTTL)
	return group, nil
}

func (s *CachedGroupStore) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	userUUID = strings.TrimSpace(userUUID)
	if groupUUID == "" || userUUID == "" || !platformBloom.GroupMayExist(groupUUID) {
		return nil, nil
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	var cached model.GroupMember
	if hit, err := platformCache.HashGetJSON(ctx, platformCache.GroupMembersKey(groupUUID), userUUID, &cached); err == nil && hit {
		return &cached, nil
	}
	member, err := s.backend.GetMember(groupUUID, userUUID)
	if err != nil || member == nil {
		return member, err
	}
	_ = platformCache.HashSetJSON(ctx, platformCache.GroupMembersKey(groupUUID), member.UserUUID, member)
	_ = platformCache.Expire(ctx, platformCache.GroupMembersKey(groupUUID), platformCache.GroupMembersTTL)
	return member, nil
}

func (s *CachedGroupStore) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" || !platformBloom.GroupMayExist(groupUUID) {
		return []*model.GroupMember{}, nil
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	if cached, err := platformCache.HashGetAll(ctx, platformCache.GroupMembersKey(groupUUID)); err == nil && len(cached) > 0 {
		members := make([]*model.GroupMember, 0, len(cached))
		for _, raw := range cached {
			var member model.GroupMember
			if err := json.Unmarshal([]byte(raw), &member); err != nil {
				members = nil
				break
			}
			members = append(members, &member)
		}
		if len(members) > 0 {
			sortGroupMembers(members)
			return members, nil
		}
	}
	members, err := s.backend.ListMembers(groupUUID)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member != nil {
			_ = platformCache.HashSetJSON(ctx, platformCache.GroupMembersKey(groupUUID), member.UserUUID, member)
		}
	}
	_ = platformCache.Expire(ctx, platformCache.GroupMembersKey(groupUUID), platformCache.GroupMembersTTL)
	sortGroupMembers(members)
	return members, nil
}

func (s *CachedGroupStore) AddMembers(groupUUID string, members []*model.GroupMember) error {
	if err := s.backend.AddMembers(groupUUID, members); err != nil {
		return err
	}
	s.invalidate(groupUUID)
	return nil
}

func (s *CachedGroupStore) Update(group *model.Group) error {
	if err := s.backend.Update(group); err != nil {
		return err
	}
	if group == nil {
		return nil
	}
	s.cacheGroup(group)
	if group.Status == model.GroupStatusDismissed {
		ctx, cancel := platformCache.NewContext()
		defer cancel()
		_ = platformCache.Delete(ctx, platformCache.GroupMembersKey(group.UUID))
	}
	return nil
}

func (s *CachedGroupStore) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	if err := s.backend.RemoveMembers(groupUUID, memberUUIDs); err != nil {
		return err
	}
	s.invalidate(groupUUID)
	return nil
}

func (s *CachedGroupStore) RemoveMember(groupUUID, userUUID string) error {
	if err := s.backend.RemoveMember(groupUUID, userUUID); err != nil {
		return err
	}
	s.invalidate(groupUUID)
	return nil
}

func (s *CachedGroupStore) cacheGroup(group *model.Group) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	_ = platformCache.SetJSON(ctx, platformCache.GroupMetaKey(group.UUID), group, platformCache.GroupMetaTTL)
}

func (s *CachedGroupStore) cacheMembers(groupUUID string, members []*model.GroupMember) {
	if len(members) == 0 {
		return
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	for _, member := range members {
		if member != nil {
			_ = platformCache.HashSetJSON(ctx, platformCache.GroupMembersKey(groupUUID), member.UserUUID, member)
		}
	}
	_ = platformCache.Expire(ctx, platformCache.GroupMembersKey(groupUUID), platformCache.GroupMembersTTL)
}

func (s *CachedGroupStore) invalidate(groupUUID string) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	_ = platformCache.Delete(
		ctx,
		platformCache.GroupMetaKey(groupUUID),
		platformCache.GroupMembersKey(groupUUID),
	)
}

func sortGroupMembers(members []*model.GroupMember) {
	sort.Slice(members, func(i, j int) bool {
		if members[i].Role != members[j].Role {
			return members[i].Role < members[j].Role
		}
		if !members[i].JoinedAt.Equal(members[j].JoinedAt) {
			return members[i].JoinedAt.Before(members[j].JoinedAt)
		}
		return members[i].UserUUID < members[j].UserUUID
	})
}
