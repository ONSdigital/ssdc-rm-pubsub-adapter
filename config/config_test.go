package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetErrorsWithoutValues(t *testing.T) {
	cfg = nil
	_, err := GetConfig()
	assert.Error(t, err, "GetConfig should return an error when in prod mode and not provided any values")
	assert.Contains(t, err.Error(), "required key")
	assert.Contains(t, err.Error(), "missing value")
}

func TestBuildRabbitConfigurationString(t *testing.T) {
	cfg = &Configuration{
		RabbitHost:       "testHost",
		RabbitPort:       "123",
		RabbitUsername:   "testUser",
		RabbitPassword:   "testPass",
		RabbitVHost:      "/testVhost",
		EqReceiptProject: "testProject",
	}

	buildRabbitConnectionString(cfg)
	assert.Equal(t, "amqp://testUser:testPass@testHost:123/testVhost", cfg.RabbitConnectionString)
}
