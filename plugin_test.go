package server

import (
	"log/slog"
	"os"
	"os/user"
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
	require.Equal(t, ids{uid: 1000, gid: 1000}, resolved)

	_, err = parseIDs(&user.User{Uid: "S-1-5-21", Gid: "1000"})
	require.ErrorContains(t, err, "failed to parse the user id")

	_, err = parseIDs(&user.User{Uid: "1000", Gid: "S-1-5-21"})
	require.ErrorContains(t, err, "failed to parse the group id")
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
