package pg

import (
	"encoding/json"
	"fmt"

	"github.com/plgd-dev/cqrs/event"
)

// Option provides the means to use function call chaining
type Option func(Config) Config

// WithMarshaler provides the possibility to set an marshaling function for the config
func WithMarshaler(f event.MarshalerFunc) Option {
	return func(cfg Config) Config {
		cfg.marshalerFunc = f
		return cfg
	}
}

// WithUnmarshaler provides the possibility to set an unmarshaling function for the config
func WithUnmarshaler(f event.UnmarshalerFunc) Option {
	return func(cfg Config) Config {
		cfg.unmarshalerFunc = f
		return cfg
	}
}

type LogDebugFunc = func(fmt string, args ...interface{})

func WithLogDebug(f LogDebugFunc) Option {
	return func(cfg Config) Config {
		cfg.logDebug = f
		return cfg
	}
}

// Config provides NATS stream configuration options
type Config struct {
	Addr     string `envconfig:"ADDRESS"  default:"localhost:26257"`
	User     string `envconfig:"USER"     default:"admin"`
	Password string `envconfig:"PASSWORD" default:""`
	Database string `envconfig:"DATABASE" default:"defaultdb"`

	marshalerFunc   event.MarshalerFunc
	unmarshalerFunc event.UnmarshalerFunc
	logDebug        LogDebugFunc
}

//String return string representation of Config
func (c Config) String() string {
	b, _ := json.MarshalIndent(c, "", "  ")
	return fmt.Sprintf("config: \n%v\n", string(b))
}
