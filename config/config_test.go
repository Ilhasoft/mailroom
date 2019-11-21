package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewConfiguration(t *testing.T) {
	expectedConfiguration := &Config{
		DB:                "postgres://temba:temba@localhost/temba?sslmode=disable",
		DBPoolSize:        36,
		Redis:             "redis://localhost:6379/15",
		Elastic:           "http://localhost:9200",
		BatchWorkers:      4,
		HandlerWorkers:    32,
		LogLevel:          "error",
		Version:           "Dev",
		SMTPServer:        "",
		MaxValueLength:    640,
		MaxStepsPerSprint: 100,
		MaxBodyBytes:      10000,
		S3Endpoint:         "https://s3.amazonaws.com",
		S3Region:           "us-east-1",
		S3MediaBucket:      "mailroom-media",
		S3MediaPrefix:      "/media/",
		S3DisableSSL:       false,
		S3ForcePathStyle:   false,
		AWSAccessKeyID:     "missing_aws_access_key_id",
		AWSSecretAccessKey: "missing_aws_secret_access_key",
		RetryPendingMessages: true,
		Address: "localhost",
		Port:    8090,
	}
	mailroomConfig := NewMailroomConfig()
	assert.Equal(t, expectedConfiguration, mailroomConfig)
	mailroomConfig.DB = "A database URL"
	assert.NotEqual(t, expectedConfiguration, mailroomConfig)
}