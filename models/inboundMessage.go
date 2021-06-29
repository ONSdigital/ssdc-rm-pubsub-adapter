package models

type InboundMessage interface {
	GetTransactionId() string
	Validate() error
}
