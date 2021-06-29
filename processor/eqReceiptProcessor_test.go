package processor

import (
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/models"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestConvertEqReceiptToRmMessage(t *testing.T) {
	timeCreated, _ := time.Parse("2006-07-08T03:04:05Z", "2008-08-24T00:00:00Z")
	eqReceiptMessage := models.EqReceipt{
		TimeCreated: &timeCreated,
		Metadata: models.EqReceiptMetadata{
			TransactionId:   "abc123xxx",
			QuestionnaireId: "01213213213",
		},
	}

	expectedRabbitMessage := models.RmMessage{
		Event: models.RmEvent{
			Type:          "RESPONSE_RECEIVED",
			Source:        "RECEIPT_SERVICE",
			Channel:       "EQ",
			DateTime:      &timeCreated,
			TransactionID: "abc123xxx",
		},
		Payload: models.RmPayload{
			Response: &models.RmResponse{
				QuestionnaireID: "01213213213",
			},
		}}

	rabbitMessage, err := convertEqReceiptToRmMessage(eqReceiptMessage)
	if err != nil {
		t.Errorf("failed: %s", err)
	}

	assert.Equal(t, expectedRabbitMessage, *rabbitMessage, "Incorrect Rabbit message structure")
}
