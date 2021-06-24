package models

import (
	"time"
)

type RmPayload struct {
	Response              *RmResponse            `json:"response,omitempty"`
}

type RmEvent struct {
	Type          string     `json:"type" validate:"required"`
	Source        string     `json:"source" validate:"required"`
	Channel       string     `json:"channel" validate:"required"`
	DateTime      *time.Time `json:"dateTime" validate:"required"`
	TransactionID string     `json:"transactionId" validate:"required"`
}

type RmMessage struct {
	Event   RmEvent   `json:"event"`
	Payload RmPayload `json:"payload"`
}
