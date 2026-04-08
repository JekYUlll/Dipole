package repository

import (
	"errors"
	"fmt"

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
