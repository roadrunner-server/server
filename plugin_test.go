package server

import (
	"os"
	"testing"

	"github.com/roadrunner-server/server/v3/test"
	"github.com/spf13/viper"
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

func TestEnv(t *testing.T) {
	log, _ := zap.NewDevelopment()
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	err := os.Setenv("MYSQL_USER", "foo")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_PASSWORD", "foo1")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_HOST", "foo2")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_PORT", "foo3")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_DATABASE", "foo4")
	require.NoError(t, err)

	v := viper.New()
	v.Set("server.command", "php ../../php_test_files/client.php echo pipes")

	m := make(map[string]interface{})
	m["env"] = `DATABASE_URL: "mysql://${MYSQL_USER}:${MYSQL_PASSWORD}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DATABASE}?serverVersion=5.7`

	v.Set("server.env", m)
	cfg, err := test.InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, log)
	require.NoError(t, err)

	for i := 0; i < len(p.preparedEnvs); i++ {
		if p.preparedEnvs[i] == `ENV=DATABASE_URL: "mysql://foo:foo1@foo2:foo3/foo4?serverVersion=5.7` {
			return
		}
	}

	t.Fatal("DATABASE_ENV not found")
}

func TestEnv2(t *testing.T) {
	log, _ := zap.NewDevelopment()
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	err := os.Setenv("MYSQL_USER", "foo")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_PASSWORD", "foo1")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_HOST", "foo2")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_PORT", "foo3")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_DATABASE", "foo4")
	require.NoError(t, err)

	v := viper.New()
	v.Set("server.command", "php ../../php_test_files/client.php echo pipes")

	m := make(map[string]interface{})
	m["env"] = `DATABASE_URL: "mysql://$MYSQL_USER:$MYSQL_PASSWORD@$MYSQL_HOST:$MYSQL_PORT/$MYSQL_DATABASE?serverVersion=5.7`

	v.Set("server.env", m)
	cfg, err := test.InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, log)
	require.NoError(t, err)

	for i := 0; i < len(p.preparedEnvs); i++ {
		if p.preparedEnvs[i] == `ENV=DATABASE_URL: "mysql://foo:foo1@foo2:foo3/foo4?serverVersion=5.7` {
			return
		}
	}

	t.Fatal("DATABASE_ENV not found")
}

func TestEnv3(t *testing.T) {
	log, _ := zap.NewDevelopment()
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	err := os.Setenv("MYSQL_USER", "foo")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_PASSWORD", "foo1")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_HOST", "foo2")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_PORT", "foo3")
	require.NoError(t, err)
	err = os.Setenv("MYSQL_DATABASE", "foo4")
	require.NoError(t, err)

	v := viper.New()
	v.Set("server.command", "php ../../php_test_files/client.php echo pipes")

	m := make(map[string]interface{})
	m["env"] = `DATABASE_URL: "mysql://$MYSQL_USE:$MYSQL_PASSWORD@$MYSQL_HOST:$MYSQL_PORT/$MYSQL_DATABASE?serverVersion=5.7`

	v.Set("server.env", m)
	cfg, err := test.InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, log)
	require.NoError(t, err)

	for i := 0; i < len(p.preparedEnvs); i++ {
		if p.preparedEnvs[i] == `ENV=DATABASE_URL: "mysql://:foo1@foo2:foo3/foo4?serverVersion=5.7` {
			return
		}
	}

	t.Fatal("DATABASE_ENV not found")
}

func TestEnv4(t *testing.T) {
	log, _ := zap.NewDevelopment()
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	v := viper.New()
	v.Set("server.command", "php ../../php_test_files/client.php echo pipes")

	m := make(map[string]interface{})
	m["env"] = `FOO: "$FOO_BAR`

	v.Set("server.env", m)
	cfg, err := test.InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, log)
	require.NoError(t, err)

	for i := 0; i < len(p.preparedEnvs); i++ {
		if p.preparedEnvs[i] == `ENV=FOO: "` {
			return
		}
	}

	t.Fatal("FOO not found")
}
