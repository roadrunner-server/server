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

func (c *Config) Unmarshal(_ interface{}) error {
	return nil
}

func (c *Config) Get(_ string) interface{} {
	return nil
}

func (c *Config) Overwrite(_ map[string]interface{}) error {
	return nil
}

func (c *Config) Has(_ string) bool {
	return true
}

func (c *Config) GracefulTimeout() time.Duration {
	return time.Second
}

func (c *Config) RRVersion() string {
	return "2.8.0"
}
