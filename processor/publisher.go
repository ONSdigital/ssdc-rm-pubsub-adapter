package processor

import (
	"context"
	"encoding/json"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/models"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

type RabbitChannel interface {
	Close() error
	Tx() error
	TxCommit() error
	TxRollback() error
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

func (p *Processor) initRabbitConnection() error {
	p.Logger.Debug("Initialising rabbit connection")
	var err error

	// Open the rabbit connection
	p.RabbitConn, err = amqp.Dial(p.Config.RabbitConnectionString)
	if err != nil {
		return errors.Wrap(err, "error connecting to rabbit")
	}
	return nil
}

func (p *Processor) initRabbitChannel(firstRabbitErr chan *amqp.Error) (RabbitChannel, error) {
	var err error
	var channel *amqp.Channel

	// Open the rabbit channel
	p.Logger.Debugf("Initialising rabbit channel no. %d", len(p.RabbitChannels)+1)
	channel, err = p.RabbitConn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "error opening rabbit channel")
	}

	if err := channel.Tx(); err != nil {
		return nil, errors.Wrap(err, "Error making rabbit channel transactional")
	}

	// Set up handler to attempt to reopen channel on channel close
	// Listen for errors on the rabbit channel to handle both channel specific and connection wide exceptions
	rabbitChannelErrs := make(chan *amqp.Error)
	go p.handleRabbitChannelErrors(rabbitChannelErrs, firstRabbitErr)
	channel.NotifyClose(rabbitChannelErrs)
	p.RabbitChannels = append(p.RabbitChannels, channel)

	return channel, nil
}

func (p *Processor) handleRabbitChannelErrors(rabbitChannelErrs <-chan *amqp.Error, firstRabbitErr chan<- *amqp.Error) {
	select {
	case rabbitErr := <-rabbitChannelErrs:
		if rabbitErr != nil {
			p.Logger.Errorw("received rabbit channel error", "error", rabbitErr)
			select {
			// This is a non-blocking channel write to trigger processor restart
			// Once the first error has been written to this channel it will asynchronously trigger a processor restart
			// so we do not care about writing any subsequent errors, they are logged then ignored.
			case firstRabbitErr <- rabbitErr:
			default:
			}
		} else {
			p.Logger.Debug("Rabbit channel shutting down")
		}
	case <-p.Context.Done():
		return
	}
}

func (p *Processor) sendProcessorErrorOnRabbitError(firstRabbitErr <-chan *amqp.Error) {
	// We only consume off this channel once to trigger the processor restart on the first error
	select {
	case err := <-firstRabbitErr:
		p.ReportError(errors.Wrap(err, "rabbit connection or channel error"))
	case <-p.Context.Done():
		return
	}
}

func (p *Processor) startPublishers(ctx context.Context) {
	// Setup one rabbit connection
	if err := p.initRabbitConnection(); err != nil {
		p.Logger.Errorw("Error initialising rabbit connection", "error", err)
		p.ReportError(err)
		return
	}

	p.RabbitChannels = make([]RabbitChannel, 0)
	firstRabbitErr := make(chan *amqp.Error)

	for i := 0; i < p.Config.PublishersPerProcessor; i++ {

		// Open a rabbit channel for each publisher worker
		channel, err := p.initRabbitChannel(firstRabbitErr)
		if err != nil {
			return
		}
		go p.Publish(ctx, channel)
	}
	go p.sendProcessorErrorOnRabbitError(firstRabbitErr)
}

func (p *Processor) Publish(ctx context.Context, channel RabbitChannel) {
	for {
		select {
		case outboundMessage := <-p.OutboundMsgChan:

			ctxLogger := p.Logger.With("transactionId", outboundMessage.EventMessage.Event.TransactionID)
			if err := p.publishEventToRabbit(outboundMessage.EventMessage, p.RabbitRoutingKey, p.Config.EventsExchange, channel); err != nil {
				ctxLogger.Errorw("Failed to publish message", "error", err)
				outboundMessage.SourceMessage.Nack()
				if err := channel.TxRollback(); err != nil {
					ctxLogger.Errorw("Error rolling back rabbit transaction after failed message publish", "error", err)
				}
				continue
			}
			if err := channel.TxCommit(); err != nil {
				ctxLogger.Errorw("Failed to commit transaction to publish message", "error", err)
				outboundMessage.SourceMessage.Nack()
				if err := channel.TxRollback(); err != nil {
					ctxLogger.Errorw("Error rolling back rabbit transaction", "error", err)
				}
				continue
			}

			outboundMessage.SourceMessage.Ack()

		case <-ctx.Done():
			return
		}
	}
}

func (p *Processor) publishEventToRabbit(message *models.RmMessage, routingKey string, exchange string, channel RabbitChannel) error {

	byteMessage, err := json.Marshal(message)
	if err != nil {
		return err
	}

	if err := channel.Publish(
		exchange,
		routingKey,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         byteMessage,
			DeliveryMode: 2, // 2 = persistent delivery mode
		}); err != nil {
		return err
	}

	p.Logger.Debugw("Published message", "routingKey", routingKey, "transactionId", message.Event.TransactionID)
	return nil
}

func (p *Processor) StopPublishers() {

}

func (p *Processor) CloseRabbit(errOk bool) {
	if p.RabbitConn != nil {
		if err := p.RabbitConn.Close(); err != nil && !errOk {
			p.Logger.Errorw("Error closing rabbit connection", "error", err)
		}
	}
}
