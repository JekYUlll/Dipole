package service

import (
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/repository"
)

var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetByUUID(uuid string) (*model.User, error) {
	user, err := s.repo.GetByUUID(uuid)
	if err != nil {
		return nil, fmt.Errorf("get user by uuid in service: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}
