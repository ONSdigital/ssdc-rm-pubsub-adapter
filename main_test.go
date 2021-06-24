// +build !unitTest

package main

// This test requires dependencies to be running

import (
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"errors"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/config"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/models"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/processor"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/readiness"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"
)

var ctx context.Context
var cfg *config.Configuration

func TestMain(m *testing.M) {
	// These tests interact with data in backing services so cannot be run in parallel
	runtime.GOMAXPROCS(1)
	ctx = context.Background()
	cfg = config.TestConfig
	code := m.Run()
	os.Exit(code)
}

func TestMessageProcessing(t *testing.T) {
	t.Run("Test EQ receipting", testMessageProcessing(
		`{"timeCreated": "2008-08-24T00:00:00Z", "metadata": {"tx_id": "abc123xxx", "questionnaire_id": "01213213213"}}`,
		`{"event":{"type":"RESPONSE_RECEIVED","source":"RECEIPT_SERVICE","channel":"EQ","dateTime":"2008-08-24T00:00:00Z","transactionId":"abc123xxx"},"payload":{"response":{"questionnaireId":"01213213213","unreceipt":false}}}`,
		cfg.EqReceiptTopic, cfg.EqReceiptProject, cfg.ReceiptRoutingKey))
}

func testMessageProcessing(messageToSend string, expectedRabbitMessage string, topic string, project string, rabbitRoutingKey string) func(t *testing.T) {
	return func(t *testing.T) {
		timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := StartProcessors(timeout, cfg, make(chan processor.Error)); err != nil {
			assert.NoError(t, err)
			return
		}

		rabbitConn, rabbitCh, err := connectToRabbitChannel()
		assert.NoError(t, err)
		defer rabbitCh.Close()
		defer rabbitConn.Close()

		if _, err := rabbitCh.QueuePurge(rabbitRoutingKey, false); err != nil {
			assert.NoError(t, err)
			return
		}

		// When
		if messageId, err := publishMessageToPubSub(timeout, messageToSend, topic, project); err != nil {
			t.Errorf("PubSub publish fail, project: %s, topic: %s, id: %s, error: %s", project, topic, messageId, err)
			return
		}

		rabbitMessage, err := getFirstMessageOnQueue(timeout, rabbitRoutingKey, rabbitCh)
		if !assert.NoErrorf(t, err, "Did not find message on queue %s", rabbitRoutingKey) {
			return
		}

		// Then
		assert.Equal(t, expectedRabbitMessage, rabbitMessage)
	}
}

func TestMessageQuarantiningBadJson(t *testing.T) {
	testMessageQuarantining("bad_message", "Test bad non JSON message is quarantined", t)
}

func TestMessageQuarantiningMissingTxnId(t *testing.T) {
	testMessageQuarantining(`{"thisMessage": "is_missing_tx_id"}`, "Test bad message missing transaction ID is quarantined", t)
}

func testMessageQuarantining(messageToSend string, testDescription string, t *testing.T) {
	var requests []*http.Request
	var requestBody []byte

	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	mockResult := "Success!"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(mockResult))
		requests = append(requests, r)
		requestBody, _ = ioutil.ReadAll(r.Body)
	}))
	defer srv.Close()

	cfg.QuarantineMessageUrl = srv.URL

	if _, err := StartProcessors(timeout, cfg, make(chan processor.Error)); err != nil {
		assert.NoError(t, err)
		return
	}

	// When
	if _, err := publishMessageToPubSub(timeout, messageToSend, cfg.EqReceiptTopic, cfg.EqReceiptProject); err != nil {
		t.Errorf("Failed to publish message to PubSub, err: %s", err)
		return
	}

	// Allow a second for the processor to process the message
	time.Sleep(1 * time.Second)

	if !assert.Len(t, requests, 1, "Unexpected number of calls to Exception Manager") {
		return
	}

	var quarantineBody models.MessageToQuarantine
	err := json.Unmarshal(requestBody, &quarantineBody)
	if err != nil {
		t.Errorf("Could not decode request body sent to Exception Manager")
		return
	}

	assert.Equal(t, "application/json", quarantineBody.ContentType, "Dodgy content type")
	assert.Equal(t, "Error unmarshalling message", quarantineBody.ExceptionClass, "Dodgy exception class")
	assert.Equal(t, messageToSend, string(quarantineBody.MessagePayload), "Dodgy message payload")
	assert.Equal(t, cfg.EqReceiptSubscription, quarantineBody.Queue, "Dodgy quarantine queue")
	assert.Equal(t, "none", quarantineBody.RoutingKey, "Dodgy routing key")
	assert.Equal(t, "Pubsub Adapter", quarantineBody.Service, "Dodgy routing key")
	assert.Len(t, quarantineBody.MessageHash, 64, "Dodgy message hash")
	assert.Len(t, quarantineBody.Headers, 1, "Dodgy headers")
	assert.Contains(t, quarantineBody.Headers, "pubSubId", "Dodgy headers, missing pubSubId")
	assert.False(t, len(quarantineBody.Headers["pubSubId"]) == 0, "Dodgy pubSubId header, expected non-zero length")
}

func TestStartProcessors(t *testing.T) {
	timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	processors, err := StartProcessors(timeout, cfg, make(chan processor.Error))
	if err != nil {
		assert.NoError(t, err)
		return
	}

	assert.Len(t, processors, 1, "StartProcessors should return 6 processors")
}

func TestRabbitReconnectOnChannelDeath(t *testing.T) {
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errChan := make(chan processor.Error)

	// Initialise up the processors normally
	processors, err := StartProcessors(timeout, cfg, errChan)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	ready := readiness.New(timeout, cfg.ReadinessFilePath)
	assert.NoError(t, ready.Ready())

	// Start the run loop
	go RunLoop(timeout, cfg, nil, errChan)

	// Take the first testProcessor
	testProcessor := processors[0]

	// Pick one of the processors rabbit channels
	channel, ok := getRabbitChannelFromProcessor(timeout, testProcessor)
	if !ok {
		assert.Fail(t, "Failed to get a rabbit channel from the processor within the timeout")
		return
	}

	// Subscribe another test channel to the close notifications
	channelErrChan := make(chan *amqp.Error)
	channel.NotifyClose(channelErrChan)

	// Check the processors rabbit channel can publish
	if err := publishToRabbit(channel, cfg.EventsExchange, cfg.ReceiptRoutingKey, `{"test":"message should publish before"}`); err != nil {
		assert.NoError(t, err)
		return
	}

	// Break the channel by publishing a mandatory message that can't be routed
	// NB: This is not a typical scenario this feature is designed around as the app or rabbit would have to be
	// mis-configured for this to occur, and the channel closing is only an undesirable side effect.
	// It is, however, the only viable way of inducing a channel close that I could think of using to exercise this code.
	if err := publishToRabbit(channel, "this_exchange_should_not_exist", cfg.ReceiptRoutingKey, `{"test":"message should fail"}`); err != nil {
		assert.NoError(t, err)
		return
	}

	// Wait for the unpublishable message to kill the channel with a timeout
	select {
	case <-timeout.Done():
		assert.Fail(t, "Timed out waiting for induced rabbit channel closure")
		return
	case <-channelErrChan:
	}

	// Try to successfully publish a message within the timeout
	success := make(chan bool)
	go attemptPublishOnProcessorsChannel(timeout, testProcessor, success)

	select {
	case <-timeout.Done():
		assert.Fail(t, "Failed to publish message with processors channel within the timeout")
		return
	case <-success:
		return
	}
}

func TestRabbitReconnectOnBadConnection(t *testing.T) {
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Induce a connection failure by using the wrong connection string
	brokenCfg := *cfg
	brokenCfg.RabbitConnectionString = "bad-connection-string"

	errChan := make(chan processor.Error)

	// Initialise up the processors normally
	processors, err := StartProcessors(timeout, &brokenCfg, errChan)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	ready := readiness.New(timeout, cfg.ReadinessFilePath)
	assert.NoError(t, ready.Ready())

	// Start the run loop
	go RunLoop(timeout, cfg, nil, errChan)

	// Take the first processor
	testProcessor := processors[0]

	// Give it a second to attempt connection and fail
	time.Sleep(1 * time.Second)

	// Fix the config
	testProcessor.Config.RabbitConnectionString = cfg.RabbitConnectionString

	// Try to successfully publish a message using the processors channel within the timeout
	success := make(chan bool)
	go attemptPublishOnProcessorsChannel(timeout, testProcessor, success)

	select {
	case <-timeout.Done():
		assert.Fail(t, "Failed to publish message with processors channel within the timeout")
		return
	case <-success:
		return
	}
}

func TestProcessorGracefullyReinitializeRabbitChannels(t *testing.T) {
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errChan := make(chan processor.Error)

	// Initialise up the processors normally
	processors, err := StartProcessors(timeout, cfg, errChan)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	// Take the first processor, for example
	testProcessor := processors[0]

	// Grab one of its rabbit channels and subscribe to its close notifications
	channel, ok := getRabbitChannelFromProcessor(timeout, testProcessor)
	if !ok {
		assert.Fail(t, "Failed to get a rabbit channel from the processor within the timeout")
		return
	}
	rabbitErrChan := make(chan *amqp.Error)
	channel.NotifyClose(rabbitErrChan)

	go RunLoop(timeout, cfg, nil, errChan)

	// Simulate an error
	errChan <- processor.Error{
		Err:       errors.New("DOOOOOOOOOOM"),
		Processor: testProcessor,
	}

	// Check the channel is gracefully shut down
	select {
	case <-timeout.Done():
		assert.Fail(t, "Timed out waiting for rabbit channel to be closed")
	case rabbitErr := <-rabbitErrChan:
		assert.Nil(t, rabbitErr, "Rabbit channel error should be nil indicating graceful shutdown")
	}

	// Ensure the processor is restarted and can publish again
	// Try to successfully publish a message using the processors channel within the timeout
	success := make(chan bool)
	go attemptPublishOnProcessorsChannel(timeout, testProcessor, success)

	select {
	case <-timeout.Done():
		assert.Fail(t, "Failed to publish message with processors channel within the timeout")
		return
	case <-success:
		return
	}

}

func getRabbitChannelFromProcessor(timeout context.Context, testProcessor *processor.Processor) (processor.RabbitChannel, bool) {
	var channel processor.RabbitChannel
	for channel == nil {
		select {
		case <-timeout.Done():
			return nil, false
		default:
			if len(testProcessor.RabbitChannels) > 0 {
				channel = testProcessor.RabbitChannels[0]
			}
		}
	}
	return channel, true
}

func publishMessageToPubSub(ctx context.Context, msg string, topic string, project string) (id string, err error) {
	pubSubClient, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return "", err
	}
	eqReceiptTopic := pubSubClient.Topic(topic)

	result := eqReceiptTopic.Publish(ctx, &pubsub.Message{
		Data: []byte(msg),
	})

	// Block until the result is returned and eqReceiptProcessor server-generated
	// ID is returned for the published message.
	return result.Get(ctx)
}

func getFirstMessageOnQueue(ctx context.Context, queue string, ch *amqp.Channel) (message string, err error) {
	msgChan, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return "", err
	}

	select {
	case d := <-msgChan:
		message = string(d.Body)
		return message, nil
	case <-ctx.Done():
		return "", errors.New("timed out waiting for the rabbit message")
	}
}

func connectToRabbitChannel() (conn *amqp.Connection, ch *amqp.Channel, err error) {
	rabbitConn, err := amqp.Dial(cfg.RabbitConnectionString)
	if err != nil {
		return nil, nil, err
	}

	rabbitChan, err := rabbitConn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return rabbitConn, rabbitChan, nil
}

func publishToRabbit(channel processor.RabbitChannel, exchange string, routingKey string, message string) error {
	return channel.Publish(
		exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         []byte(message),
			DeliveryMode: 2, // 2 = persistent delivery mode
		})
}

func attemptPublishOnProcessorsChannel(ctx context.Context, testProcessor *processor.Processor, success chan bool) {
	for {
		select {
		case <-ctx.Done():
			// Kill this goroutine if the test times out
			return
		default:
			// Repeatedly try to publish a message using the processors channel
			if len(testProcessor.RabbitChannels) == 0 {
				continue
			}
			channel := testProcessor.RabbitChannels[0]
			if err := publishToRabbit(channel, cfg.EventsExchange, cfg.ReceiptRoutingKey, `{"test":"message should publish after"}`); err == nil {
				// We have successfully published a message with the processors re-opened rabbit channel
				success <- true
				return
			}
		}
	}
}
