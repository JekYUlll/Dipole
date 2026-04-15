package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
	"github.com/JekYUlll/Dipole/internal/store"
)

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) Create(user *model.User) error {
	if err := store.DB.Create(user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	if user != nil {
		platformBloom.AddUser(user.UUID)
		ctx, cancel := platformCache.NewContext()
		defer cancel()
		_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(user.UUID), user, platformCache.UserProfileTTL)
	}

	return nil
}

func (r *UserRepository) UpsertAssistant(user *model.User) error {
	if user == nil {
		return nil
	}

	assignments := map[string]any{
		"nickname":      user.Nickname,
		"telephone":     user.Telephone,
		"email":         user.Email,
		"avatar":        user.Avatar,
		"password_hash": user.PasswordHash,
		"is_admin":      user.IsAdmin,
		"user_type":     user.UserType,
		"status":        user.Status,
	}
	if err := store.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uuid"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(user).Error; err != nil {
		return fmt.Errorf("upsert assistant user: %w", err)
	}

	platformBloom.AddUser(user.UUID)
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(user.UUID), user, platformCache.UserProfileTTL)

	return nil
}

func (r *UserRepository) GetByUUID(uuid string) (*model.User, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return nil, nil
	}
	if !platformBloom.UserMayExist(uuid) {
		return nil, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	var cached model.User
	if hit, err := platformCache.GetJSON(ctx, platformCache.UserProfileKey(uuid), &cached); err == nil && hit {
		return &cached, nil
	}

	var user model.User
	if err := store.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get user by uuid: %w", err)
	}

	_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(uuid), &user, platformCache.UserProfileTTL)
	return &user, nil
}

func (r *UserRepository) GetByTelephone(telephone string) (*model.User, error) {
	var user model.User
	if err := store.DB.Where("telephone = ?", telephone).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get user by telephone: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) Update(user *model.User) error {
	if err := store.DB.Save(user).Error; err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if user != nil {
		ctx, cancel := platformCache.NewContext()
		defer cancel()
		_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(user.UUID), user, platformCache.UserProfileTTL)
	}

	return nil
}

func (r *UserRepository) SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error) {
	query := store.DB.Model(&model.User{}).Where("status = ?", model.UserStatusNormal)
	query = applyUserKeywordFilter(query, keyword)
	if excludeUUID != "" {
		query = query.Where("uuid <> ?", excludeUUID)
	}

	users, err := listUsers(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search active users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) List(keyword string, status *int8, limit int) ([]*model.User, error) {
	query := store.DB.Model(&model.User{})
	query = applyUserKeywordFilter(query, keyword)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	users, err := listUsers(query, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) ListByUUIDs(uuids []string) ([]*model.User, error) {
	normalized := normalizeUUIDs(uuids)
	if len(normalized) == 0 {
		return []*model.User{}, nil
	}
	normalized = filterExistingUserUUIDs(normalized)
	if len(normalized) == 0 {
		return []*model.User{}, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	usersByUUID := make(map[string]*model.User, len(normalized))
	missing := make([]string, 0, len(normalized))
	for _, uuid := range normalized {
		var cached model.User
		if hit, err := platformCache.GetJSON(ctx, platformCache.UserProfileKey(uuid), &cached); err == nil && hit {
			usersByUUID[uuid] = &cached
			continue
		}
		missing = append(missing, uuid)
	}

	if len(missing) > 0 {
		var users []*model.User
		if err := store.DB.Where("uuid IN ?", missing).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("list users by uuids: %w", err)
		}
		for _, user := range users {
			usersByUUID[user.UUID] = user
			_ = platformCache.SetJSON(ctx, platformCache.UserProfileKey(user.UUID), user, platformCache.UserProfileTTL)
		}
	}

	result := make([]*model.User, 0, len(usersByUUID))
	for _, uuid := range normalized {
		if user := usersByUUID[uuid]; user != nil {
			result = append(result, user)
		}
	}

	return result, nil
}

func normalizeUUIDs(uuids []string) []string {
	seen := make(map[string]struct{}, len(uuids))
	normalized := make([]string, 0, len(uuids))
	for _, uuid := range uuids {
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			continue
		}
		if _, ok := seen[uuid]; ok {
			continue
		}
		seen[uuid] = struct{}{}
		normalized = append(normalized, uuid)
	}

	return normalized
}

func filterExistingUserUUIDs(uuids []string) []string {
	filtered := make([]string, 0, len(uuids))
	for _, uuid := range uuids {
		if platformBloom.UserMayExist(uuid) {
			filtered = append(filtered, uuid)
		}
	}

	return filtered
}

func applyUserKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return query
	}

	pattern := "%" + keyword + "%"
	return query.Where(
		"uuid LIKE ? OR telephone LIKE ? OR nickname LIKE ?",
		pattern,
		pattern,
		pattern,
	)
}

func listUsers(query *gorm.DB, limit int) ([]*model.User, error) {
	var users []*model.User
	if err := query.Order("created_at DESC").Limit(limit).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}
