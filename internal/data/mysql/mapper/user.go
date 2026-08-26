package mapper

import (
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func UserCreateParams(user *model.User) generated.CreateUserParams {
	return generated.CreateUserParams{
		Uuid:           user.UUID,
		Nickname:       user.Nickname,
		Telephone:      user.Telephone,
		Email:          userString(user.Email),
		Avatar:         user.Avatar,
		AvatarFileUuid: userString(user.AvatarFileUUID),
		Signature:      user.Signature,
		PasswordHash:   user.PasswordHash,
		IsAdmin:        user.IsAdmin,
		UserType:       user.UserType,
		Status:         user.Status,
	}
}

func AssistantUserUpsertParams(user *model.User) generated.UpsertAssistantUserParams {
	return generated.UpsertAssistantUserParams{
		Uuid:           user.UUID,
		Nickname:       user.Nickname,
		Telephone:      user.Telephone,
		Email:          userString(user.Email),
		Avatar:         user.Avatar,
		AvatarFileUuid: userString(user.AvatarFileUUID),
		Signature:      user.Signature,
		PasswordHash:   user.PasswordHash,
		IsAdmin:        user.IsAdmin,
		UserType:       user.UserType,
		Status:         user.Status,
	}
}

func UserUpdateParams(user *model.User) generated.UpdateUserParams {
	return generated.UpdateUserParams{
		Nickname:       user.Nickname,
		Telephone:      user.Telephone,
		Email:          userString(user.Email),
		Avatar:         user.Avatar,
		AvatarFileUuid: userString(user.AvatarFileUUID),
		Signature:      user.Signature,
		PasswordHash:   user.PasswordHash,
		IsAdmin:        user.IsAdmin,
		UserType:       user.UserType,
		Status:         user.Status,
		Uuid:           user.UUID,
	}
}

func User(row generated.User) *model.User {
	return &model.User{
		ID:             uint(row.ID),
		UUID:           row.Uuid,
		Nickname:       row.Nickname,
		Telephone:      row.Telephone,
		Email:          row.Email.String,
		Avatar:         row.Avatar,
		AvatarFileUUID: row.AvatarFileUuid.String,
		Signature:      row.Signature,
		PasswordHash:   row.PasswordHash,
		IsAdmin:        row.IsAdmin,
		UserType:       row.UserType,
		Status:         row.Status,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func Users(rows []generated.User) []*model.User {
	users := make([]*model.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, User(row))
	}
	return users
}

func userString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}
