package processor

import (
	"context"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/config"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/models"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestProcessor_Publish_Success(t *testing.T) {
	// Given
	testProcessor := newTestProcessor()
	mockSourceMessage := new(MockPubSubMessage)
	mockChannel := new(MockRabbitChannel)

	mockChannel.On("Publish",
		testProcessor.Config.EventsExchange,
		testProcessor.RabbitRoutingKey,
		true,  // Mandatory
		false, // Immediate
		mock.Anything).Return(nil)
	mockChannel.On("TxCommit").Return(nil)
	mockSourceMessage.On("Ack").Once()

	outboundMessage := models.OutboundMessage{
		EventMessage: &models.RmMessage{
			Event:   models.RmEvent{},
			Payload: models.RmPayload{},
		},
		SourceMessage: mockSourceMessage,
	}

	// When
	// Send it a message to publish
	go func() { testProcessor.OutboundMsgChan <- &outboundMessage }()

	// Run the publisher within a timeout period
	timeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	testProcessor.Publish(timeout, mockChannel)

	// Then
	mockChannel.AssertExpectations(t)
	mockSourceMessage.AssertExpectations(t)
}

func TestProcessor_Publish_PublishFailure(t *testing.T) {
	// Given
	testProcessor := newTestProcessor()
	mockSourceMessage := new(MockPubSubMessage)
	mockChannel := new(MockRabbitChannel)

	// Simulate failure to publish the first time it tries to commit, success the second
	mockChannel.On("Publish",
		testProcessor.Config.EventsExchange,
		testProcessor.RabbitRoutingKey,
		true,  // Mandatory
		false, // Immediate
		mock.Anything).Return(errors.New("Publishing failed")).Once()
	mockChannel.On("TxRollback").Return(nil)
	mockSourceMessage.On("Nack").Once()
	mockChannel.On("Publish",
		testProcessor.Config.EventsExchange,
		testProcessor.RabbitRoutingKey,
		true,  // Mandatory
		false, // Immediate
		mock.Anything).Return(nil).Once()
	mockChannel.On("TxCommit").Return(nil).Once()
	mockSourceMessage.On("Ack").Once()

	outboundMessage := models.OutboundMessage{
		EventMessage: &models.RmMessage{
			Event:   models.RmEvent{},
			Payload: models.RmPayload{},
		},
		SourceMessage: mockSourceMessage,
	}

	// When
	// Send the message in twice to simulate redelivery
	go func() {
		testProcessor.OutboundMsgChan <- &outboundMessage
		testProcessor.OutboundMsgChan <- &outboundMessage
	}()

	// Run the publisher within a timeout period
	timeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	testProcessor.Publish(timeout, mockChannel)

	// Then
	mockChannel.AssertExpectations(t)
	mockSourceMessage.AssertExpectations(t)
}

func TestProcessor_Publish_CommitFailure(t *testing.T) {
	// Given
	testProcessor := newTestProcessor()
	mockSourceMessage := new(MockPubSubMessage)
	mockChannel := new(MockRabbitChannel)

	// Simulate failure to commit the first time it tries to commit, success the second
	mockChannel.On("Publish",
		testProcessor.Config.EventsExchange,
		testProcessor.RabbitRoutingKey,
		true,
		false,
		mock.Anything).Return(nil).Twice()
	mockChannel.On("TxCommit").Return(errors.New("Commit failed")).Once()
	mockChannel.On("TxRollback").Return(nil).Once()
	mockSourceMessage.On("Nack").Once()
	mockChannel.On("TxCommit").Return(nil).Once()
	mockSourceMessage.On("Ack").Once()

	outboundMessage := models.OutboundMessage{
		EventMessage: &models.RmMessage{
			Event:   models.RmEvent{},
			Payload: models.RmPayload{},
		},
		SourceMessage: mockSourceMessage,
	}

	// When
	// We send the message in twice to simulate redelivery
	go func() {
		testProcessor.OutboundMsgChan <- &outboundMessage
		testProcessor.OutboundMsgChan <- &outboundMessage
	}()

	// Run the publisher within a timeout period
	timeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	testProcessor.Publish(timeout, mockChannel)

	// Then
	mockChannel.AssertExpectations(t)
	mockSourceMessage.AssertExpectations(t)
}

// Helpers
func newTestProcessor() *Processor {
	return &Processor{
		Logger:           zap.S(),
		Config:           config.TestConfig,
		OutboundMsgChan:  make(chan *models.OutboundMessage),
		RabbitRoutingKey: "test-routing-key",
	}
}

// Mocks
type MockPubSubMessage struct {
	mock.Mock
}

func (m *MockPubSubMessage) Ack() {
	m.Called()
}
func (m *MockPubSubMessage) Nack() {
	m.Called()
}

type MockRabbitChannel struct {
	mock.Mock
}

func (m *MockRabbitChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRabbitChannel) Tx() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRabbitChannel) TxCommit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRabbitChannel) TxRollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRabbitChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	args := m.Called(exchange, key, mandatory, immediate, msg)
	return args.Error(0)
}

func (m *MockRabbitChannel) NotifyClose(value chan *amqp.Error) chan *amqp.Error {
	m.Called(value)
	return value
}
