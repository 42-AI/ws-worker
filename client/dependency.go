package client

import (
	"github.com/42-AI/ws-worker/conf"
	"github.com/42-AI/ws-worker/internal/logger"
	"go.uber.org/zap"
	"log"
)

func dependencies() (*logger.Logger, conf.Configuration, *DockerHandler, error) {
	lg, err := logger.ProvideLogger()
	if err != nil {
		log.Fatalf("cannot create logger %v", err)
	}

	cf, err := conf.GetConfig(lg)
	if err != nil {
		lg.Error("cannot get config", zap.Error(err))
	}

	dh := &DockerHandler{}
	err = dh.new(lg, cf)
	if err != nil {
		lg.Error("cannot get DockerHandler", zap.Error(err))
	}

	return lg, cf, dh, nil
}
