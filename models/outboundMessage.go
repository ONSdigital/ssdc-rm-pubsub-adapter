package models

type OutboundMessage struct {
	EventMessage  *RmMessage
	SourceMessage PubSubMessage
}

type PubSubMessage interface {
	Ack()
	Nack()
}
