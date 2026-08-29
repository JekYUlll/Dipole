package coreapplication

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type assistantUserStore interface {
	UpsertAssistant(*model.User) error
}

// EnsureAIAssistantUser creates or updates the system assistant identity when
// AI is enabled. The operation belongs to Core because Core owns user data.
func EnsureAIAssistantUser(users assistantUserStore) error {
	if users == nil {
		return fmt.Errorf("assistant user store is required")
	}
	cfg := config.AIConfig()
	if !cfg.Enabled {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("dipole-ai-assistant"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("generate ai assistant password hash: %w", err)
	}
	assistant := &model.User{
		UUID: cfg.AssistantUUID, Nickname: cfg.AssistantNickname,
		Telephone: cfg.AssistantTelephone, Email: cfg.AssistantEmail,
		Avatar: cfg.AssistantAvatar, PasswordHash: string(passwordHash),
		UserType: model.UserTypeAssistant, Status: model.UserStatusNormal,
	}
	if assistant.Avatar == "" {
		assistant.Avatar = model.DefaultAvatarURL
	}
	if err := users.UpsertAssistant(assistant); err != nil {
		return err
	}
	logger.Info("ai assistant user ensured", zap.String("assistant_uuid", assistant.UUID), zap.String("provider", cfg.Provider), zap.String("model", cfg.Model))
	return nil
}
