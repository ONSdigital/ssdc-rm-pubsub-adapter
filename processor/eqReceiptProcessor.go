package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/config"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/models"
)

func NewEqReceiptProcessor(ctx context.Context, appConfig *config.Configuration, errChan chan Error) (*Processor, error) {
	return NewProcessor(ctx, appConfig, appConfig.EqReceiptProject, appConfig.EqReceiptSubscription, appConfig.ReceiptRoutingKey, convertEqReceiptToRmMessage, unmarshalEqReceipt, errChan)
}

func unmarshalEqReceipt(data []byte) (models.InboundMessage, error) {
	var eqReceipt models.EqReceipt
	if err := json.Unmarshal(data, &eqReceipt); err != nil {
		return nil, err
	}
	if err := eqReceipt.Validate(); err != nil {
		return nil, err
	}

	return eqReceipt, nil
}

func convertEqReceiptToRmMessage(receipt models.InboundMessage) (*models.RmMessage, error) {
	eqReceipt, ok := receipt.(models.EqReceipt)
	if !ok {
		return nil, fmt.Errorf("wrong message model given to convertEqReceiptToRmMessage: %T, only accepts EqReceipt, tx_id: %q", receipt, receipt.GetTransactionId())
	}

	return &models.RmMessage{
		Event: models.RmEvent{
			Type:          "RESPONSE_RECEIVED",
			Source:        "RECEIPT_SERVICE",
			Channel:       "EQ",
			DateTime:      eqReceipt.TimeCreated,
			TransactionID: eqReceipt.Metadata.TransactionId,
		},
		Payload: models.RmPayload{
			Response: &models.RmResponse{
				QuestionnaireID: eqReceipt.Metadata.QuestionnaireId,
				CaseID:          eqReceipt.Metadata.CaseID,
			},
		},
	}, nil
}
