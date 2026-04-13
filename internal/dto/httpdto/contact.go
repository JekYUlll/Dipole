package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type ApplyContactRequest struct {
	TargetUUID string `json:"target_uuid" binding:"required"`
	Message    string `json:"message" binding:"omitempty,max=255"`
}

func (r ApplyContactRequest) ToInput() service.ApplyContactInput {
	return service.ApplyContactInput{
		TargetUUID: r.TargetUUID,
		Message:    r.Message,
	}
}

type HandleContactApplicationRequest struct {
	Action string `json:"action" binding:"required,oneof=accept reject"`
}

type ContactResponse struct {
	User      *PublicUserResponse `json:"user"`
	CreatedAt time.Time           `json:"created_at"`
}

func ToContactResponses(items []*service.ContactListItem) []*ContactResponse {
	response := make([]*ContactResponse, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		response = append(response, &ContactResponse{
			User:      ToPublicUserResponse(item.User),
			CreatedAt: item.CreatedAt,
		})
	}

	return response
}

type ContactApplicationResponse struct {
	ID        uint                `json:"id"`
	Status    int8                `json:"status"`
	Message   string              `json:"message"`
	Applicant *PublicUserResponse `json:"applicant"`
	Target    *PublicUserResponse `json:"target"`
	HandledAt *time.Time          `json:"handled_at,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
}

func ToContactApplicationResponses(items []*service.ContactApplicationView) []*ContactApplicationResponse {
	response := make([]*ContactApplicationResponse, 0, len(items))
	for _, item := range items {
		if item == nil || item.Application == nil {
			continue
		}
		response = append(response, &ContactApplicationResponse{
			ID:        item.Application.ID,
			Status:    item.Application.Status,
			Message:   item.Application.Message,
			Applicant: ToPublicUserResponse(item.Applicant),
			Target:    ToPublicUserResponse(item.Target),
			HandledAt: item.Application.HandledAt,
			CreatedAt: item.Application.CreatedAt,
		})
	}

	return response
}

func ToContactApplicationResponse(item *model.ContactApplication, applicant, target *model.User) *ContactApplicationResponse {
	if item == nil {
		return nil
	}

	return &ContactApplicationResponse{
		ID:        item.ID,
		Status:    item.Status,
		Message:   item.Message,
		Applicant: ToPublicUserResponse(applicant),
		Target:    ToPublicUserResponse(target),
		HandledAt: item.HandledAt,
		CreatedAt: item.CreatedAt,
	}
}
