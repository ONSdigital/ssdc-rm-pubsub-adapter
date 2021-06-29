package config

import (
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
)

type Configuration struct {
	ReadinessFilePath           string `envconfig:"READINESS_FILE_PATH" default:"/tmp/pubsub-adapter-ready"`
	LogLevel                    string `envconfig:"LOG_LEVEL" default:"ERROR"`
	QuarantineMessageUrl        string `envconfig:"QUARANTINE_MESSAGE_URL"  required:"true"`
	PublishersPerProcessor      int    `envconfig:"PUBLISHERS_PER_PROCESSOR" default:"20"`
	ProcessorRestartWaitSeconds int    `envconfig:"PROCESSOR_RESTART_WAIT_SECONDS" default:"5"`
	ProcessorStartUpTimeSeconds int    `envconfig:"PROCESSOR_START_UP_TIME_SECONDS" default:"5"`

	// Rabbit
	RabbitHost             string `envconfig:"RABBIT_HOST" required:"true"`
	RabbitPort             string `envconfig:"RABBIT_PORT" required:"true"`
	RabbitUsername         string `envconfig:"RABBIT_USERNAME" required:"true"`
	RabbitPassword         string `envconfig:"RABBIT_PASSWORD"  required:"true"  json:"-"`
	RabbitVHost            string `envconfig:"RABBIT_VHOST"  default:"/"`
	RabbitConnectionString string `json:"-"`
	EventsExchange         string `envconfig:"RABBIT_EXCHANGE"  default:"events"`
	ReceiptRoutingKey      string `envconfig:"RECEIPT_ROUTING_KEY"  default:"event.caseProcessor.receipt"`

	// PubSub
	EqReceiptProject      string `envconfig:"EQ_RECEIPT_PROJECT" required:"true"`
	EqReceiptSubscription string `envconfig:"EQ_RECEIPT_SUBSCRIPTION" default:"rm-receipt-subscription"`
	EqReceiptTopic        string `envconfig:"EQ_RECEIPT_TOPIC" default:"eq-submission-topic"`
}

var cfg *Configuration
var TestConfig = &Configuration{
	PublishersPerProcessor:      1,
	ProcessorRestartWaitSeconds: 1,
	ProcessorStartUpTimeSeconds: 1,
	ReadinessFilePath:           "/tmp/pubsub-adapter-ready",
	RabbitConnectionString:      "amqp://guest:guest@localhost:7672/",
	ReceiptRoutingKey:           "goTestReceiptQueue",
	EqReceiptProject:            "project",
	EqReceiptSubscription:       "rm-receipt-subscription",
	EqReceiptTopic:              "eq-submission-topic",
}

func GetConfig() (*Configuration, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Configuration{}
	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, err
	}

	buildRabbitConnectionString(cfg)

	return cfg, nil
}

// String is implemented to prevent sensitive fields being logged.
// The config is returned as JSON with sensitive fields omitted.
func (config Configuration) String() string {
	jsonConfig, _ := json.Marshal(config)
	return string(jsonConfig)
}

func buildRabbitConnectionString(cfg *Configuration) {
	if cfg.RabbitConnectionString == "" {
		cfg.RabbitConnectionString = fmt.Sprintf("amqp://%s:%s@%s:%s%s",
			cfg.RabbitUsername, cfg.RabbitPassword, cfg.RabbitHost, cfg.RabbitPort, cfg.RabbitVHost)
	}
}
