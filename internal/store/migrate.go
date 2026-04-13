package store

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
)

func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("mysql not initialized")
	}

	if err := DB.AutoMigrate(
		&model.User{},
		&model.Message{},
		&model.Conversation{},
		&model.Contact{},
		&model.ContactApplication{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	return nil
}
