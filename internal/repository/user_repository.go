package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/JekYUlll/Dipole/internal/model"
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

	return nil
}

func (r *UserRepository) GetByUUID(uuid string) (*model.User, error) {
	var user model.User
	if err := store.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get user by uuid: %w", err)
	}

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
