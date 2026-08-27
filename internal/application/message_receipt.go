package application

import "github.com/JekYUlll/Dipole/internal/model"

type MessageCommandReceiptStatus string

const (
	MessageCommandReceiptStatusAbsent    MessageCommandReceiptStatus = "absent"
	MessageCommandReceiptStatusCommitted MessageCommandReceiptStatus = "committed"
)

type MessageCommandReceipt struct {
	Status  MessageCommandReceiptStatus
	Message *model.Message
}

type MessageCommandReceiptQuery interface {
	GetMessageCommandReceipt(senderUUID, clientMessageID string) (*MessageCommandReceipt, error)
}
