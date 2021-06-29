package models

type MessageToQuarantine struct {
	MessageHash      string            `json:"messageHash"`
	MessagePayload   []byte            `json:"messagePayload"`
	Service          string            `json:"service"`
	Queue            string            `json:"queue"`
	ExceptionClass   string            `json:"exceptionClass"`
	ExceptionMessage string            `json:"exceptionMessage"`
	RoutingKey       string            `json:"routingKey"`
	ContentType      string            `json:"contentType"`
	Headers          map[string]string `json:"headers"`
}
