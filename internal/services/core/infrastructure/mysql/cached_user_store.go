package coremysql

import (
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
)

var _ application.UserStore = (*CachedUserStore)(nil)

type CachedUserStore struct {
	backend application.UserStore
}

func NewCachedUserStore(backend application.UserStore) *CachedUserStore {
	if backend == nil {
		panic("cached user store backend is required")
	}
	return &CachedUserStore{backend: backend}
}

func (s *CachedUserStore) Create(user *model.User) error {
	if err := s.backend.Create(user); err != nil {
		return err
	}
	s.cache(user, true)
	return nil
}

func (s *CachedUserStore) UpsertAssistant(user *model.User) error {
	if err := s.backend.UpsertAssistant(user); err != nil {
		return err
	}
	s.cache(user, true)
	return nil
}

func (s *CachedUserStore) GetByUUID(uuid string) (*model.User, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" || !platformBloom.UserMayExist(uuid) {
		return nil, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()
	var cached model.User
	if hit, err := platformCache.GetJSON(ctx, platformCache.UserProfileKey(uuid), &cached); err == nil && hit {
		return &cached, nil
	}

	user, err := s.backend.GetByUUID(uuid)
	if err != nil || user == nil {
		return user, err
	}
	_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(uuid), user, platformCache.UserProfileTTL)
	return user, nil
}

func (s *CachedUserStore) GetByTelephone(telephone string) (*model.User, error) {
	return s.backend.GetByTelephone(telephone)
}

func (s *CachedUserStore) Update(user *model.User) error {
	if err := s.backend.Update(user); err != nil {
		return err
	}
	s.cache(user, false)
	return nil
}

func (s *CachedUserStore) SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error) {
	return s.backend.SearchActive(keyword, excludeUUID, limit)
}

func (s *CachedUserStore) List(keyword string, status *int8, limit int) ([]*model.User, error) {
	return s.backend.List(keyword, status, limit)
}

func (s *CachedUserStore) ListByUUIDs(uuids []string) ([]*model.User, error) {
	normalized := normalizeCachedUserUUIDs(uuids)
	if len(normalized) == 0 {
		return []*model.User{}, nil
	}

	filtered := normalized[:0]
	for _, uuid := range normalized {
		if platformBloom.UserMayExist(uuid) {
			filtered = append(filtered, uuid)
		}
	}
	if len(filtered) == 0 {
		return []*model.User{}, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()
	usersByUUID := make(map[string]*model.User, len(filtered))
	missing := make([]string, 0, len(filtered))
	for _, uuid := range filtered {
		var cached model.User
		if hit, err := platformCache.GetJSON(ctx, platformCache.UserProfileKey(uuid), &cached); err == nil && hit {
			usersByUUID[uuid] = &cached
			continue
		}
		missing = append(missing, uuid)
	}

	if len(missing) > 0 {
		users, err := s.backend.ListByUUIDs(missing)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			if user == nil {
				continue
			}
			usersByUUID[user.UUID] = user
			_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(user.UUID), user, platformCache.UserProfileTTL)
		}
	}

	result := make([]*model.User, 0, len(usersByUUID))
	for _, uuid := range filtered {
		if user := usersByUUID[uuid]; user != nil {
			result = append(result, user)
		}
	}
	return result, nil
}

func (s *CachedUserStore) cache(user *model.User, addToBloom bool) {
	if user == nil {
		return
	}
	if addToBloom {
		platformBloom.AddUser(user.UUID)
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(user.UUID), user, platformCache.UserProfileTTL)
}

func normalizeCachedUserUUIDs(uuids []string) []string {
	seen := make(map[string]struct{}, len(uuids))
	normalized := make([]string, 0, len(uuids))
	for _, uuid := range uuids {
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			continue
		}
		if _, exists := seen[uuid]; exists {
			continue
		}
		seen[uuid] = struct{}{}
		normalized = append(normalized, uuid)
	}
	return normalized
}
