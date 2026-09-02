package coremysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.UserStore = (*UserRepository)(nil)

type UserRepository struct {
	queries generated.Querier
}

func NewUserRepository(queries generated.Querier) (*UserRepository, error) {
	if queries == nil {
		return nil, errors.New("user queries are required")
	}
	return &UserRepository{queries: queries}, nil
}

func (r *UserRepository) Create(user *model.User) error {
	if user == nil {
		return errors.New("create user with sqlc: user is required")
	}
	if _, err := r.queries.CreateUser(context.Background(), mapper.UserCreateParams(user)); err != nil {
		return fmt.Errorf("create user with sqlc: %w", err)
	}
	return r.reload(user)
}

func (r *UserRepository) UpsertAssistant(user *model.User) error {
	if user == nil {
		return nil
	}
	if _, err := r.queries.UpsertAssistantUser(context.Background(), mapper.AssistantUserUpsertParams(user)); err != nil {
		return fmt.Errorf("upsert assistant user with sqlc: %w", err)
	}
	return r.reload(user)
}

func (r *UserRepository) GetByUUID(uuid string) (*model.User, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return nil, nil
	}
	row, err := r.queries.GetUserByUUID(context.Background(), uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by UUID with sqlc: %w", err)
	}
	return mapper.User(row), nil
}

func (r *UserRepository) GetByTelephone(telephone string) (*model.User, error) {
	row, err := r.queries.GetUserByTelephone(context.Background(), telephone)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by telephone with sqlc: %w", err)
	}
	return mapper.User(row), nil
}

func (r *UserRepository) Update(user *model.User) error {
	if user == nil {
		return errors.New("update user with sqlc: user is required")
	}
	if _, err := r.queries.UpdateUser(context.Background(), mapper.UserUpdateParams(user)); err != nil {
		return fmt.Errorf("update user with sqlc: %w", err)
	}
	return r.reload(user)
}

func (r *UserRepository) SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error) {
	rows, err := r.queries.SearchActiveUsers(context.Background(), generated.SearchActiveUsersParams{
		Status:      model.UserStatusNormal,
		Pattern:     userSearchPattern(keyword),
		ExcludeUuid: excludeUUID,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search active users with sqlc: %w", err)
	}
	return mapper.Users(rows), nil
}

func (r *UserRepository) List(keyword string, status *int8, limit int) ([]*model.User, error) {
	var (
		rows []generated.User
		err  error
	)
	if status == nil {
		rows, err = r.queries.ListUsers(context.Background(), generated.ListUsersParams{
			Pattern: userSearchPattern(keyword),
			Limit:   int32(limit),
		})
	} else {
		rows, err = r.queries.ListUsersByStatus(context.Background(), generated.ListUsersByStatusParams{
			Status:  *status,
			Pattern: userSearchPattern(keyword),
			Limit:   int32(limit),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("list users with sqlc: %w", err)
	}
	return mapper.Users(rows), nil
}

func (r *UserRepository) ListByUUIDs(uuids []string) ([]*model.User, error) {
	normalized := normalizeUserUUIDs(uuids)
	if len(normalized) == 0 {
		return []*model.User{}, nil
	}
	rows, err := r.queries.ListUsersByUUIDs(context.Background(), normalized)
	if err != nil {
		return nil, fmt.Errorf("list users by UUIDs with sqlc: %w", err)
	}
	usersByUUID := make(map[string]*model.User, len(rows))
	for _, user := range mapper.Users(rows) {
		usersByUUID[user.UUID] = user
	}
	users := make([]*model.User, 0, len(usersByUUID))
	for _, uuid := range normalized {
		if user := usersByUUID[uuid]; user != nil {
			users = append(users, user)
		}
	}
	return users, nil
}

func (r *UserRepository) reload(user *model.User) error {
	stored, err := r.GetByUUID(user.UUID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("persisted user was not found")
	}
	*user = *stored
	return nil
}

func userSearchPattern(keyword string) string {
	return "%" + strings.TrimSpace(keyword) + "%"
}

func normalizeUserUUIDs(uuids []string) []string {
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
