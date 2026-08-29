package coreauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
)

var (
	ErrInvalidToken         = errors.New("invalid token")
	ErrInvalidAgentMCPGrant = errors.New("invalid Agent MCP grant")
)

const (
	AgentMCPResource  = application.AgentMCPResource
	AgentMCPReadScope = application.AgentMCPReadScope

	sessionTokenUse  = "session"
	agentMCPTokenUse = "agent_mcp_access"
	agentMCPTokenTTL = 15 * time.Minute
)

type tokenClaims struct {
	jwt.RegisteredClaims
	Scope    string `json:"scope,omitempty"`
	TokenUse string `json:"token_use,omitempty"`
}

type TokenSession struct {
	UserUUID  string
	TokenID   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

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
	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.UUID,
			Issuer:    authCfg.JWTIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        jti,
		},
		TokenUse: sessionTokenUse,
	}
	return signToken(claims, secret)
}

func (s *TokenService) IssueAgentMCPAccessToken(userUUID, resource string, scopes []string, consent bool) (string, error) {
	userUUID = strings.TrimSpace(userUUID)
	expectedResource := AgentMCPResourceIdentifier()
	if ValidateAgentMCPResource(expectedResource) != nil || !consent || userUUID == "" || strings.TrimSpace(resource) != expectedResource || len(scopes) != 1 || strings.TrimSpace(scopes[0]) != AgentMCPReadScope {
		return "", ErrInvalidAgentMCPGrant
	}
	authCfg := config.AuthConfig()
	secret := strings.TrimSpace(authCfg.JWTSecret)
	if secret == "" {
		return "", errors.New("jwt secret is empty")
	}
	jti, err := generateTokenID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userUUID, Issuer: authCfg.JWTIssuer, Audience: jwt.ClaimStrings{expectedResource},
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(agentMCPTokenTTL)), ID: jti,
		},
		Scope: AgentMCPReadScope, TokenUse: agentMCPTokenUse,
	}
	return signToken(claims, secret)
}

func (s *TokenService) Resolve(token string) (string, error) {
	session, err := s.ResolveSession(token)
	if err != nil {
		return "", err
	}

	return session.UserUUID, nil
}

func (s *TokenService) ResolveSession(token string) (*TokenSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrInvalidToken
	}

	claims, err := s.parseClaims(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Tokens issued before token_use was introduced remain valid during rollout.
	if claims.TokenUse != "" && claims.TokenUse != sessionTokenUse {
		return nil, ErrInvalidToken
	}
	if err := validateTokenState(claims); err != nil {
		return nil, ErrInvalidToken
	}

	session := &TokenSession{
		UserUUID:  claims.Subject,
		TokenID:   claims.ID,
		ExpiresAt: claims.ExpiresAt.Time.UTC(),
	}
	if claims.IssuedAt != nil {
		session.IssuedAt = claims.IssuedAt.Time.UTC()
	}

	return session, nil
}

func (s *TokenService) ResolveAgentMCPAccessToken(token, resource, requiredScope string) (*TokenSession, error) {
	claims, err := s.parseClaims(strings.TrimSpace(token))
	if err != nil || claims.TokenUse != agentMCPTokenUse || strings.TrimSpace(resource) == "" || !audienceContains(claims.Audience, resource) || !scopeContains(claims.Scope, requiredScope) {
		return nil, ErrInvalidToken
	}
	if err := validateTokenState(claims); err != nil {
		return nil, ErrInvalidToken
	}
	session := &TokenSession{UserUUID: claims.Subject, TokenID: claims.ID, ExpiresAt: claims.ExpiresAt.Time.UTC()}
	if claims.IssuedAt != nil {
		session.IssuedAt = claims.IssuedAt.Time.UTC()
	}
	return session, nil
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

	return s.RevokeTokenID(claims.ID, claims.ExpiresAt.Time)
}

func (s *TokenService) RevokeTokenID(tokenID string, expiresAt time.Time) error {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return ErrInvalidToken
	}
	if expiresAt.IsZero() {
		return ErrInvalidToken
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := platformCache.SetString(ctx, revokedTokenKey(tokenID), "1", ttl); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}

func (s *TokenService) parseClaims(rawToken string) (*tokenClaims, error) {
	authCfg := config.AuthConfig()
	secret := strings.TrimSpace(authCfg.JWTSecret)
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	claims := &tokenClaims{}
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

func signToken(claims tokenClaims, secret string) (string, error) {
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign jwt token: %w", err)
	}
	return signed, nil
}

func validateTokenState(claims *tokenClaims) error {
	if claims == nil || strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.ID) == "" || claims.ExpiresAt == nil {
		return ErrInvalidToken
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	revoked, err := platformCache.Exists(ctx, revokedTokenKey(claims.ID))
	if err != nil || revoked {
		return ErrInvalidToken
	}
	return nil
}

func scopeContains(scope, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return false
	}
	for _, candidate := range strings.Fields(scope) {
		if candidate == required {
			return true
		}
	}
	return false
}

func audienceContains(audience jwt.ClaimStrings, required string) bool {
	for _, candidate := range audience {
		if candidate == required {
			return true
		}
	}
	return false
}

func AgentMCPResourceIdentifier() string {
	return application.AgentMCPResourceIdentifier(config.AuthConfig().AgentMCPResource)
}

func ValidateAgentMCPResource(resource string) error {
	if err := application.ValidateAgentMCPResource(resource); err != nil {
		return ErrInvalidAgentMCPGrant
	}
	return nil
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
