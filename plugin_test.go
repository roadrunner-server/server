package server

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCommandUnknownUser(t *testing.T) {
	require.Panics(t, func() {
		log, _ := zap.NewDevelopment()
		p := &Plugin{
			preparedEnvs: make([]string, 0),
			cfg:          &Config{User: "foo"},
			log:          log,
		}

		_ = p.customCmd(nil)("php foo/bar")
	})
}

func TestCommand1(t *testing.T) {
	log, _ := zap.NewDevelopment()
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	cmd := p.customCmd(nil)("php foo/bar")
	require.Equal(t, "php", cmd.Args[0])
	require.Equal(t, "foo/bar", cmd.Args[1])
}

func TestCommand2(t *testing.T) {
	log, _ := zap.NewDevelopment()
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	cmd := p.customCmd(nil)("php foo bar")
	require.Equal(t, "php", cmd.Args[0])
	require.Equal(t, "foo", cmd.Args[1])
	require.Equal(t, "bar", cmd.Args[2])
}
