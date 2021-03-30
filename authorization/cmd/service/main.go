package main

import (
	"context"

	"github.com/plgd-dev/cloud/authorization/service"
	"github.com/plgd-dev/cloud/pkg/config"
	"github.com/plgd-dev/cloud/pkg/log"
)

func main() {
	var cfg service.Config
	err := config.LoadAndValidateConfig(&cfg)
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}
	logger, err := log.NewLogger(cfg.Log)
	if err != nil {
		log.Fatalf("cannot create logger: %v", err)
	}
	log.Set(logger)
	s, err := service.New(context.Background(), cfg, logger)
	if err != nil {
		log.Fatalf("cannot create service: %v", err)
	}
	err = s.Serve()
	if err != nil {
		log.Fatalf("cannot serve service: %v", err)
	}
}
