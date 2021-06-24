package processor

import (
	"bytes"
	"cloud.google.com/go/pubsub"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/config"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/logger"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/models"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"net/http"
)

type messageUnmarshaller func([]byte) (models.InboundMessage, error)

type messageConverter func(message models.InboundMessage) (*models.RmMessage, error)

type Error struct {
	Err error
	*Processor
}

type Processor struct {
	Name                 string
	RabbitConn           *amqp.Connection
	RabbitRoutingKey     string
	RabbitChannels       []RabbitChannel
	OutboundMsgChan      chan *models.OutboundMessage
	Config               *config.Configuration
	PubSubProject        string
	PubSubSubscriptionId string
	PubSubClient         *pubsub.Client
	PubSubSubscription   *pubsub.Subscription
	unmarshallMessage    messageUnmarshaller
	convertMessage       messageConverter
	ErrChan              chan Error
	Logger               *zap.SugaredLogger
	Context              context.Context
	Cancel               context.CancelFunc
}

func NewProcessor(ctx context.Context,
	appConfig *config.Configuration,
	pubSubProject string,
	pubSubSubscription string,
	routingKey string,
	messageConverter messageConverter,
	messageUnmarshaller messageUnmarshaller, errChan chan Error) (*Processor, error) {
	p := &Processor{}
	p.Name = pubSubSubscription + ">" + routingKey
	p.PubSubSubscriptionId = pubSubSubscription
	p.PubSubProject = pubSubProject
	p.Config = appConfig
	p.RabbitRoutingKey = routingKey
	p.convertMessage = messageConverter
	p.unmarshallMessage = messageUnmarshaller
	p.ErrChan = errChan
	p.OutboundMsgChan = make(chan *models.OutboundMessage)
	p.RabbitChannels = make([]RabbitChannel, 0)
	p.Logger = logger.Logger.With("processor", p.Name)

	if err := p.Initialise(ctx); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Processor) Initialise(ctx context.Context) (err error) {
	// Set up context
	p.Context, p.Cancel = context.WithCancel(ctx)

	// Initialise PubSub
	if err := p.initPubSub(); err != nil {
		return err
	}

	// Initialise consuming from PubSub
	p.Logger.Infow("Launching PubSub message receiver")
	go p.Consume()

	// Initialise and manage publisher workers
	go p.startPublishers(ctx)
	return nil
}

func (p *Processor) initPubSub() (err error) {
	// Set up PubSub connection
	p.PubSubClient, err = pubsub.NewClient(p.Context, p.PubSubProject)
	if err != nil {
		return errors.Wrap(err, "error settings up PubSub client")
	}

	// Set up PubSub subscription
	p.PubSubSubscription = p.PubSubClient.Subscription(p.PubSubSubscriptionId)
	return nil
}

func (p *Processor) Consume() {
	err := p.PubSubSubscription.Receive(p.Context, p.Process)
	if err != nil {
		p.Logger.Errorw("Error in consumer", "error", err)
		p.ReportError(err)
	}
}

func (p *Processor) Process(_ context.Context, msg *pubsub.Message) {
	ctxLogger := p.Logger.With("msgId", msg.ID)
	messageReceived, err := p.unmarshallMessage(msg.Data)
	if err != nil {
		ctxLogger.Errorw("Error unmarshalling message, quarantining", "error", err, "data", string(msg.Data))
		if err := p.quarantineMessage(msg, err); err != nil {
			ctxLogger.Errorw("Error quarantining bad message, nacking", "error", err, "data", string(msg.Data))
			msg.Nack()
			return
		}
		ctxLogger.Debugw("Acking quarantined message", "msgData", string(msg.Data))
		msg.Ack()
		return
	}
	ctxLogger = ctxLogger.With("transactionId", messageReceived.GetTransactionId())
	ctxLogger.Debugw("Processing message")
	rmMessageToSend, err := p.convertMessage(messageReceived)
	if err != nil {
		ctxLogger.Errorw("Error converting message", "error", err)
		return
	}
	ctxLogger.Debugw("Sending outbound message to publish", "msgData", string(msg.Data))
	p.OutboundMsgChan <- &models.OutboundMessage{SourceMessage: msg, EventMessage: rmMessageToSend}
}

func (p *Processor) quarantineMessage(message *pubsub.Message, rootErr error) error {
	headers := map[string]string{
		"pubSubId": message.ID,
	}

	for key, value := range message.Attributes {
		headers[key] = value
	}

	msgToQuarantine := models.MessageToQuarantine{
		MessageHash:      fmt.Sprintf("%x", sha256.Sum256(message.Data)),
		MessagePayload:   message.Data,
		Service:          "Pubsub Adapter",
		Queue:            p.PubSubSubscription.ID(),
		ExceptionClass:   "Error unmarshalling message",
		ExceptionMessage: rootErr.Error(),
		RoutingKey:       "none",
		ContentType:      "application/json",
		Headers:          headers,
	}

	jsonValue, err := json.Marshal(msgToQuarantine)
	if err != nil {
		return err
	}

	_, err = http.Post(p.Config.QuarantineMessageUrl, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}

	p.Logger.Debugw("Quarantined message", "data", string(message.Data))
	return nil
}

func (p *Processor) Stop() {
	p.Logger.Debug("Stopping processor")
	p.Cancel()
	p.CloseRabbit(false)
}

func (p *Processor) Restart(ctx context.Context) {
	p.Stop()
	if err := p.Initialise(ctx); err != nil {
		logger.Logger.Errorw("Failed to restart processor", "error", err, "processor", p.Name)
		p.ReportError(err)
	}

}

func (p *Processor) ReportError(err error) {
	// Writing to a channel is a blocking operation, it waits till there is a consumer ready.
	// Do it in a go routine to ensure it gets written without blocking the caller.
	go func() {
		p.ErrChan <- Error{
			Err:       err,
			Processor: p,
		}
	}()
}
