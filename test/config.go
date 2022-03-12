package test

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	v *viper.Viper
}

func InitMockCfg(v *viper.Viper) (*Config, error) {
	return &Config{
		v: v,
	}, nil
}

func (c *Config) UnmarshalKey(name string, out interface{}) error {
	return c.v.UnmarshalKey(name, out)
}

func (c *Config) Unmarshal(out interface{}) error {
	return nil
}

func (c *Config) Get(name string) interface{} {
	return nil
}

func (c *Config) Overwrite(values map[string]interface{}) error {
	return nil
}

func (c *Config) Has(name string) bool {
	return true
}

func (c *Config) GracefulTimeout() time.Duration {
	return time.Second
}

func (c *Config) RRVersion() string {
	return "2.8.0"
}
