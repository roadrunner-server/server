package server

import (
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/tcplisten"
)

// Config All config (.rr.yaml)
// For other section use pointer to distinguish between `empty` and `not present`
type Config struct {
	// OnInit configuration
	OnInit *InitConfig `mapstructure:"on_init"`
	// Command to run as application.
	Command []string `mapstructure:"command"`
	// User to run application under.
	User string `mapstructure:"user"`
	// Group to run application under.
	Group string `mapstructure:"group"`
	// Env represents application environment.
	Env map[string]string `mapstructure:"env"`
	// Relay defines connection method and factory to be used to connect to workers:
	// "pipes", "tcp://:6001", "unix://rr.sock"
	// This config section must not change on re-configuration.
	Relay string `mapstructure:"relay"`
	// RelaySocket sets permissions and ownership for a filesystem UNIX relay socket.
	RelaySocket *tcplisten.UnixSocketOptions `mapstructure:"relay_socket"`
}

type InitConfig struct {
	// Command which is started before worker starts
	Command []string `mapstructure:"command"`
	// ExecTimeout is execute timeout for the command
	ExecTimeout time.Duration `mapstructure:"exec_timeout"`
	// Env represents application environment.
	Env map[string]string `mapstructure:"env"`
	// Env represents UID
	User string `mapstructure:"user"`
	// ExitOnError defines if the RR should exit if the command fails
	ExitOnError bool `mapstructure:"exit_on_error"`
}

// RPCConfig should be in sync with rpc/config.go
// Used to set RPC address env
type RPCConfig struct {
	Listen string `mapstructure:"listen"`
}

// InitDefaults for the server config
func (cfg *Config) InitDefaults() error {
	if len(cfg.Command) == 0 {
		return errors.Str("command should not be empty")
	}

	if cfg.Relay == "" {
		cfg.Relay = "pipes"
	}

	if err := cfg.RelaySocket.Validate(cfg.Relay); err != nil {
		return errors.E(errors.Op("server.relay_socket"), err)
	}

	if cfg.OnInit != nil {
		if len(cfg.OnInit.Command) == 0 {
			return errors.Str("on_init command should not be empty")
		}

		if cfg.OnInit.ExecTimeout == 0 {
			cfg.OnInit.ExecTimeout = time.Minute
		}
	}

	return nil
}

// Check ownership values before Viper converts them to integers.
func validateRelaySocketIDs(cfg Configurer) error {
	const key = "server.relay_socket"
	if !cfg.Has(key) {
		return nil
	}

	var raw map[string]any
	if err := cfg.UnmarshalKey(key, &raw); err != nil {
		return errors.E(errors.Op(key), err)
	}

	for _, field := range []string{"uid", "gid"} {
		if raw[field] == nil {
			continue
		}

		value := reflect.ValueOf(raw[field])
		valid := false
		switch value.Kind() { //nolint:exhaustive // Other kinds are invalid ownership values.
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			id := value.Int()
			valid = id >= 0 && id < 1<<32-1
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			valid = value.Uint() < 1<<32-1
		case reflect.String:
			id, err := strconv.ParseInt(value.String(), 0, strconv.IntSize)
			valid = err == nil && id >= 0 && id < 1<<32-1
		case reflect.Float32, reflect.Float64:
			id := value.Float()
			valid = id >= 0 && id < 1<<32-1 && id == math.Trunc(id)
		default:
			valid = false
		}

		if !valid {
			return errors.Errorf("%s.%s: must be an integer from 0 to 4294967294", key, field)
		}
	}

	return nil
}
