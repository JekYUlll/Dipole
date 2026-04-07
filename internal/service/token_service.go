package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

var ErrInvalidToken = errors.New("invalid token")

type TokenService struct{}

func NewTokenService() *TokenService {
	return &TokenService{}
}

func (s *TokenService) Issue(user *model.User) (string, error) {
	token, err := generateAccessToken()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ttl := time.Duration(config.AuthConfig().TokenTTLHours) * time.Hour
	if err := store.RDB.Set(ctx, tokenKey(token), user.UUID, ttl).Err(); err != nil {
		return "", fmt.Errorf("save token: %w", err)
	}

	return token, nil
}

func (s *TokenService) Resolve(token string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	userUUID, err := store.RDB.Get(ctx, tokenKey(token)).Result()
	if err != nil {
		return "", ErrInvalidToken
	}

	return userUUID, nil
}

func (s *TokenService) Revoke(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := store.RDB.Del(ctx, tokenKey(token)).Err(); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}

func generateAccessToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

func tokenKey(token string) string {
	return "auth:token:" + token
}
