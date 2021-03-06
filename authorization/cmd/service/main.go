package main

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"github.com/plgd-dev/cloud/authorization/persistence/mongodb"
	"github.com/plgd-dev/cloud/authorization/provider"
	"github.com/plgd-dev/cloud/authorization/service"
	"github.com/plgd-dev/kit/log"
	"github.com/plgd-dev/kit/security/certManager"
)

func main() {
	var cfg service.Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatalf("cannot parse config: %v", err)
	}

	log.Setup(cfg.Log)
	log.Info(cfg.String())

	dialCertManager, err := certManager.NewCertManager(cfg.Dial)
	if err != nil {
		log.Fatalf("cannot parse config: %v", err)
	}

	tlsConfig := dialCertManager.GetClientTLSConfig()

	persistence, err := mongodb.NewStore(context.Background(), cfg.MongoDB, mongodb.WithTLS(tlsConfig))
	if err != nil {
		log.Fatalf("cannot parse config: %v", err)
	}
	if cfg.Device.OAuth2.AccessType == "" {
		cfg.Device.OAuth2.AccessType = "offline"
	}
	if cfg.SDK.AccessType == "" {
		cfg.SDK.AccessType = "online"
	}
	if cfg.Device.OAuth2.ResponseType == "" {
		cfg.Device.OAuth2.ResponseType = "code"
	}
	if cfg.Device.OAuth2.ResponseMode == "" {
		cfg.Device.OAuth2.ResponseMode = "query"
	}
	if cfg.SDK.ResponseType == "" {
		cfg.SDK.ResponseType = "token"
	}
	if cfg.SDK.ResponseMode == "" {
		cfg.SDK.ResponseMode = "query"
	}
	deviceProvider := provider.New(cfg.Device, tlsConfig)
	sdkProvider := provider.New(provider.Config{
		Provider: "generic",
		OAuth2:   cfg.SDK,
	}, tlsConfig)
	s, err := service.New(cfg, persistence, deviceProvider, sdkProvider)
	if err != nil {
		log.Fatalf("cannot parse config: %v", err)
	}
	s.Serve()
}
