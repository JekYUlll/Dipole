package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

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
	authCfg := config.AuthConfig()
	secret := strings.TrimSpace(authCfg.JWTSecret)
	if secret == "" {
		return "", errors.New("jwt secret is empty")
	}

	jti, err := generateTokenID()
	if err != nil {
		return "", err
	}

	ttl := time.Duration(config.AuthConfig().TokenTTLHours) * time.Hour
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   user.UUID,
		Issuer:    authCfg.JWTIssuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		ID:        jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign jwt token: %w", err)
	}

	return signed, nil
}

func (s *TokenService) Resolve(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrInvalidToken
	}

	claims, err := s.parseClaims(token)
	if err != nil {
		return "", ErrInvalidToken
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	revoked, err := store.RDB.Exists(ctx, revokedTokenKey(claims.ID)).Result()
	if err != nil {
		return "", ErrInvalidToken
	}
	if revoked > 0 {
		return "", ErrInvalidToken
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return "", ErrInvalidToken
	}

	return claims.Subject, nil
}

func (s *TokenService) Revoke(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrInvalidToken
	}

	claims, err := s.parseClaims(token)
	if err != nil {
		return ErrInvalidToken
	}
	if strings.TrimSpace(claims.ID) == "" {
		return ErrInvalidToken
	}
	if claims.ExpiresAt == nil {
		return ErrInvalidToken
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := store.RDB.Set(ctx, revokedTokenKey(claims.ID), "1", ttl).Err(); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}

func (s *TokenService) parseClaims(rawToken string) (*jwt.RegisteredClaims, error) {
	authCfg := config.AuthConfig()
	secret := strings.TrimSpace(authCfg.JWTSecret)
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	claims := &jwt.RegisteredClaims{}
	parsedToken, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsedToken.Valid {
		return nil, ErrInvalidToken
	}

	issuer := strings.TrimSpace(authCfg.JWTIssuer)
	if issuer != "" && claims.Issuer != issuer {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func generateTokenID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}

	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

func revokedTokenKey(tokenID string) string {
	return "auth:revoked:" + tokenID
}
