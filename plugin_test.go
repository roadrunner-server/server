package server

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"slices"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type Cfg struct {
	v *viper.Viper
}

func InitMockCfg(v *viper.Viper) (*Cfg, error) {
	return &Cfg{
		v: v,
	}, nil
}

func (c *Cfg) UnmarshalKey(name string, out any) error {
	return c.v.UnmarshalKey(name, out)
}

func (c *Cfg) Unmarshal(_ any) error {
	return nil
}

func (c *Cfg) Get(_ string) any {
	return nil
}

func (c *Cfg) Overwrite(_ map[string]any) error {
	return nil
}

func (c *Cfg) Has(_ string) bool {
	return true
}

func (c *Cfg) GracefulTimeout() time.Duration {
	return time.Second
}

func (c *Cfg) RRVersion() string {
	return "2.8.0"
}

type TestLogger struct {
	log *slog.Logger
}

func NewTestLogger(log *slog.Logger) *TestLogger {
	return &TestLogger{
		log: log,
	}
}

func (tl *TestLogger) NamedLogger(string) *slog.Logger {
	return tl.log
}

func TestCommandUnknownUser(t *testing.T) {
	require.Panics(t, func() {
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		p := &Plugin{
			preparedEnvs: make([]string, 0),
			cfg:          &Config{User: "foo"},
			log:          log,
		}

		_ = p.customCmd(nil)([]string{"php foo/bar"})
	})
}

func TestInitResolvesUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("server.user is not supported on windows")
	}

	current, err := user.Current()
	require.NoError(t, err)

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	v := viper.New()
	v.Set("server.command", "php php_test_files/client.php echo pipes")
	v.Set("server.user", current.Username)

	cfg, err := InitMockCfg(v)
	require.NoError(t, err)
	require.NoError(t, p.Init(cfg, NewTestLogger(log)))

	uid, err := strconv.Atoi(current.Uid)
	require.NoError(t, err)
	gid, err := strconv.Atoi(current.Gid)
	require.NoError(t, err)

	require.Equal(t, uid, p.UID())
	require.Equal(t, gid, p.GID())
}

func TestParseIDs(t *testing.T) {
	resolved, err := parseIDs(&user.User{Uid: "1000", Gid: "1000"})
	require.NoError(t, err)
	require.Equal(t, &ids{uid: 1000, gid: 1000}, resolved)

	resolved, err = parseIDs(&user.User{Uid: "S-1-5-21", Gid: "1000"})
	require.ErrorContains(t, err, "failed to parse the user id")
	require.Nil(t, resolved)

	resolved, err = parseIDs(&user.User{Uid: "1000", Gid: "S-1-5-21"})
	require.ErrorContains(t, err, "failed to parse the group id")
	require.Nil(t, resolved)
}

func TestInitUnknownUser(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	v := viper.New()
	v.Set("server.command", "php php_test_files/client.php echo pipes")
	v.Set("server.user", "rr-definitely-missing-user")

	cfg, err := InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, NewTestLogger(log))
	require.Error(t, err)
	// the failed resolution must leave the ids unset, reading as 0/0
	require.Equal(t, 0, p.UID())
	require.Equal(t, 0, p.GID())
}

func TestCommand1(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	cmd := p.customCmd(nil)([]string{"php foo/bar"})
	require.Equal(t, "php", cmd.Args[0])
	require.Equal(t, "foo/bar", cmd.Args[1])
}

func TestCommand2(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	cmd := p.customCmd(nil)([]string{"php foo bar"})
	require.Equal(t, "php", cmd.Args[0])
	require.Equal(t, "foo", cmd.Args[1])
	require.Equal(t, "bar", cmd.Args[2])
}

func TestCommand3(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	cmd := p.customCmd(nil)([]string{"php", "foo/bar"})
	require.Equal(t, "php", cmd.Args[0])
	require.Equal(t, "foo/bar", cmd.Args[1])
}

func TestCommand4_spaces(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	cmd := p.customCmd(nil)([]string{"/Application Support/folder/php", "foo/bar"})
	require.Equal(t, "/Application Support/folder/php", cmd.Args[0])
	require.Equal(t, "foo/bar", cmd.Args[1])
}

func TestEnv(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
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
	v.Set("server.command", "php php_test_files/client.php echo pipes")

	m := make(map[string]any)
	m["env"] = `DATABASE_URL: "mysql://${MYSQL_USER}:${MYSQL_PASSWORD}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DATABASE}?serverVersion=5.7`

	v.Set("server.env", m)
	cfg, err := InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, NewTestLogger(log))
	require.NoError(t, err)

	if slices.Contains(p.preparedEnvs, `ENV=DATABASE_URL: "mysql://foo:foo1@foo2:foo3/foo4?serverVersion=5.7`) {
		return
	}

	t.Fatal("DATABASE_ENV not found")
}

func TestEnv2(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
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
	v.Set("server.command", "php php_test_files/client.php echo pipes")

	m := make(map[string]any)
	m["env"] = `DATABASE_URL: "mysql://$MYSQL_USER:$MYSQL_PASSWORD@$MYSQL_HOST:$MYSQL_PORT/$MYSQL_DATABASE?serverVersion=5.7`

	v.Set("server.env", m)
	cfg, err := InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, NewTestLogger(log))
	require.NoError(t, err)

	if slices.Contains(p.preparedEnvs, `ENV=DATABASE_URL: "mysql://foo:foo1@foo2:foo3/foo4?serverVersion=5.7`) {
		return
	}

	t.Fatal("DATABASE_ENV not found")
}

func TestEnv3(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
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
	v.Set("server.command", "php php_test_files/client.php echo pipes")

	m := make(map[string]any)
	m["env"] = `DATABASE_URL: "mysql://$MYSQL_USE:$MYSQL_PASSWORD@$MYSQL_HOST:$MYSQL_PORT/$MYSQL_DATABASE?serverVersion=5.7`

	v.Set("server.env", m)
	cfg, err := InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, NewTestLogger(log))
	require.NoError(t, err)

	if slices.Contains(p.preparedEnvs, `ENV=DATABASE_URL: "mysql://:foo1@foo2:foo3/foo4?serverVersion=5.7`) {
		return
	}

	t.Fatal("DATABASE_ENV not found")
}

func TestEnv4(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &Plugin{
		preparedEnvs: make([]string, 0),
		cfg:          &Config{},
		log:          log,
	}

	v := viper.New()
	v.Set("server.command", "php php_test_files/client.php echo pipes")

	m := make(map[string]any)
	m["env"] = `FOO: "$FOO_BAR`

	v.Set("server.env", m)
	cfg, err := InitMockCfg(v)
	require.NoError(t, err)

	err = p.Init(cfg, NewTestLogger(log))
	require.NoError(t, err)

	if slices.Contains(p.preparedEnvs, `ENV=FOO: "`) {
		return
	}

	t.Fatal("FOO not found")
}

func TestName(t *testing.T) {
	require.Equal(t, PluginName, (&Plugin{}).Name())
}

// TestUIDGIDWithoutUser covers the nil guard: with no user configured the
// plugin reports 0 rather than dereferencing the unset ids.
func TestUIDGIDWithoutUser(t *testing.T) {
	p := &Plugin{}

	require.Equal(t, 0, p.UID())
	require.Equal(t, 0, p.GID())
}

func TestUIDGIDWithResolvedUser(t *testing.T) {
	p := &Plugin{ids: &ids{uid: 1234, gid: 5678}}

	require.Equal(t, 1234, p.UID())
	require.Equal(t, 5678, p.GID())
}

func TestConfigInitDefaults(t *testing.T) {
	t.Run("command is required", func(t *testing.T) {
		require.ErrorContains(t, (&Config{}).InitDefaults(), "command should not be empty")
	})

	t.Run("relay defaults to pipes", func(t *testing.T) {
		cfg := &Config{Command: []string{"php", "worker.php"}}
		require.NoError(t, cfg.InitDefaults())
		require.Equal(t, "pipes", cfg.Relay)
	})

	t.Run("relay is left alone when set", func(t *testing.T) {
		cfg := &Config{Command: []string{"php", "worker.php"}, Relay: "tcp://127.0.0.1:9999"}
		require.NoError(t, cfg.InitDefaults())
		require.Equal(t, "tcp://127.0.0.1:9999", cfg.Relay)
	})

	t.Run("on_init command is required", func(t *testing.T) {
		cfg := &Config{Command: []string{"php", "worker.php"}, OnInit: &InitConfig{}}
		require.ErrorContains(t, cfg.InitDefaults(), "on_init command should not be empty")
	})

	t.Run("on_init exec timeout defaults to a minute", func(t *testing.T) {
		cfg := &Config{
			Command: []string{"php", "worker.php"},
			OnInit:  &InitConfig{Command: []string{"php", "init.php"}},
		}
		require.NoError(t, cfg.InitDefaults())
		require.Equal(t, time.Minute, cfg.OnInit.ExecTimeout)
	})

	t.Run("on_init exec timeout is left alone when set", func(t *testing.T) {
		cfg := &Config{
			Command: []string{"php", "worker.php"},
			OnInit:  &InitConfig{Command: []string{"php", "init.php"}, ExecTimeout: time.Second * 5},
		}
		require.NoError(t, cfg.InitDefaults())
		require.Equal(t, time.Second*5, cfg.OnInit.ExecTimeout)
	})
}

// TestCommandWriteForwardsToLogger covers the io.Writer the on_init command's
// output is piped through.
func TestCommandWriteForwardsToLogger(t *testing.T) {
	var buf bytes.Buffer
	c := newCommand(slog.New(slog.NewTextHandler(&buf, nil)), &InitConfig{})

	n, err := c.Write([]byte("hello from on_init"))

	require.NoError(t, err)
	require.Equal(t, len("hello from on_init"), n)
	require.Contains(t, buf.String(), "hello from on_init")
}

// TestCreateProcessAppliesEnv checks config env is uppercased, expanded and
// appended after the OS environment so it wins.
func TestCreateProcessAppliesEnv(t *testing.T) {
	t.Setenv("SERVER_TEST_BASE", "expanded")

	c := newCommand(slog.New(slog.NewTextHandler(io.Discard, nil)), &InitConfig{})
	cmd := c.createProcess(map[string]string{"lower_key": "${SERVER_TEST_BASE}-value"}, []string{"php", "worker.php"})

	require.Equal(t, "php", filepath.Base(cmd.Path))
	require.Equal(t, []string{"php", "worker.php"}, cmd.Args)
	require.Contains(t, cmd.Env, "LOWER_KEY=expanded-value")
}

// TestCreateProcessSingleArgument covers the branch where the command carries
// no arguments.
func TestCreateProcessSingleArgument(t *testing.T) {
	c := newCommand(slog.New(slog.NewTextHandler(io.Discard, nil)), &InitConfig{})
	cmd := c.createProcess(nil, []string{"php"})

	require.Equal(t, []string{"php"}, cmd.Args)
}
