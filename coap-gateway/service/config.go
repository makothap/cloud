package service

import (
	"time"

	"github.com/plgd-dev/cloud/pkg/security/oauth/manager"
	"github.com/plgd-dev/cloud/resource-aggregate/cqrs/eventbus/nats"
	"github.com/plgd-dev/kit/sync/task/queue"
)

type DeviceStatusExpirationConfig struct {
	Enabled   bool          `yaml:"enabled" json:"enabled"`
	ExpiresIn time.Duration `yaml:"expiresIn" json:"expiresIn"`
}

func (c *DeviceStatusExpirationConfig) SetDefaults() {
	if c.ExpiresIn == 0 {
		c.ExpiresIn = 24 * time.Hour
	}
}

// Config for application.
type Config struct {
	Addr                            string         `envconfig:"ADDRESS" default:"0.0.0.0:5684"`
	ExternalPort                    uint16         `envconfig:"EXTERNAL_PORT" default:"5684"`
	FQDN                            string         `envconfig:"FQDN" default:"coapgw.ocf.cloud"`
	AuthServerAddr                  string         `envconfig:"AUTH_SERVER_ADDRESS" default:"127.0.0.1:9100"`
	ResourceAggregateAddr           string         `envconfig:"RESOURCE_AGGREGATE_ADDRESS"  default:"127.0.0.1:9100"`
	ResourceDirectoryAddr           string         `envconfig:"RESOURCE_DIRECTORY_ADDRESS"  default:"127.0.0.1:9100"`
	RequestTimeout                  time.Duration  `envconfig:"REQUEST_TIMEOUT"  default:"10s"`
	KeepaliveEnable                 bool           `envconfig:"KEEPALIVE_ENABLE" default:"true"`
	KeepaliveTimeoutConnection      time.Duration  `envconfig:"KEEPALIVE_TIMEOUT_CONNECTION" default:"20s"`
	DisableBlockWiseTransfer        bool           `envconfig:"DISABLE_BLOCKWISE_TRANSFER" default:"false"`
	BlockWiseTransferSZX            string         `envconfig:"BLOCKWISE_TRANSFER_SZX" default:"1024"`
	DisableTCPSignalMessageCSM      bool           `envconfig:"DISABLE_TCP_SIGNAL_MESSAGE_CSM"  default:"false"`
	DisablePeerTCPSignalMessageCSMs bool           `envconfig:"DISABLE_PEER_TCP_SIGNAL_MESSAGE_CSMS"  default:"false"`
	SendErrorTextInResponse         bool           `envconfig:"ERROR_IN_RESPONSE"  default:"true"`
	OAuth                           manager.Config `envconfig:"OAUTH"`
	ReconnectInterval               time.Duration  `envconfig:"RECONNECT_TIMEOUT" default:"10s"`
	HeartBeat                       time.Duration  `envconfig:"HEARTBEAT" default:"4s"`
	MaxMessageSize                  int            `envconfig:"MAX_MESSAGE_SIZE" default:"262144"`
	LogMessages                     bool           `envconfig:"LOG_MESSAGES" default:"false"`
	DeviceStatusExpiration          DeviceStatusExpirationConfig
	TaskQueue                       queue.Config
	Nats                            nats.Config
}

func (c *Config) SetDefaults() {
	c.TaskQueue.SetDefaults()
	c.DeviceStatusExpiration.SetDefaults()
}
