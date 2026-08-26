package mapper

import (
	"database/sql"
	"time"

	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func Contact(row generated.Contact) *model.Contact {
	return &model.Contact{
		ID:         uint(row.ID),
		UserUUID:   row.UserUuid,
		FriendUUID: row.FriendUuid,
		Remark:     row.Remark,
		Status:     row.Status,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func Contacts(rows []generated.Contact) []*model.Contact {
	contacts := make([]*model.Contact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, Contact(row))
	}
	return contacts
}

func ContactApplicationCreateParams(contactApplication *model.ContactApplication) generated.CreateContactApplicationParams {
	return generated.CreateContactApplicationParams{
		ApplicantUuid: contactApplication.ApplicantUUID,
		TargetUuid:    contactApplication.TargetUUID,
		Message:       contactApplication.Message,
		Status:        contactApplication.Status,
		ExpiresAt:     contactNullableTime(contactApplication.ExpiresAt),
		HandledAt:     contactNullableTime(contactApplication.HandledAt),
	}
}

func ContactApplicationUpdateParams(contactApplication *model.ContactApplication) generated.UpdateContactApplicationParams {
	return generated.UpdateContactApplicationParams{
		Message:   contactApplication.Message,
		Status:    contactApplication.Status,
		ExpiresAt: contactNullableTime(contactApplication.ExpiresAt),
		HandledAt: contactNullableTime(contactApplication.HandledAt),
		ID:        uint64(contactApplication.ID),
	}
}

func ContactApplication(row generated.ContactApplication) *model.ContactApplication {
	return &model.ContactApplication{
		ID:            uint(row.ID),
		ApplicantUUID: row.ApplicantUuid,
		TargetUUID:    row.TargetUuid,
		Message:       row.Message,
		Status:        row.Status,
		ExpiresAt:     contactTimePointer(row.ExpiresAt),
		HandledAt:     contactTimePointer(row.HandledAt),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}

func ContactApplications(rows []generated.ContactApplication) []*model.ContactApplication {
	contactApplications := make([]*model.ContactApplication, 0, len(rows))
	for _, row := range rows {
		contactApplications = append(contactApplications, ContactApplication(row))
	}
	return contactApplications
}

func contactNullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

func contactTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
