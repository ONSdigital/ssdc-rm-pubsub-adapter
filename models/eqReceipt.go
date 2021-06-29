package models

import (
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/validate"
	"time"
)

type EqReceiptMetadata struct {
	TransactionId   string `json:"tx_id" validate:"required"`
	QuestionnaireId string `json:"questionnaire_id" validate:"required"`
	CaseID          string `json:"caseId,omitempty"`
}

type EqReceipt struct {
	TimeCreated *time.Time        `json:"timeCreated" validate:"required"`
	Metadata    EqReceiptMetadata `json:"metadata" validate:"required"`
}

func (e EqReceipt) GetTransactionId() string {
	return e.Metadata.TransactionId
}

func (e EqReceipt) Validate() error {
	return validate.Validate.Struct(e)
}
