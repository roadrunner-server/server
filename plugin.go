package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v4/ipc/pipe"
	"github.com/roadrunner-server/sdk/v4/ipc/socket"
	"github.com/roadrunner-server/sdk/v4/payload"
	"go.uber.org/zap"

	"github.com/roadrunner-server/sdk/v4/pool"
	staticPool "github.com/roadrunner-server/sdk/v4/pool/static_pool"
	"github.com/roadrunner-server/sdk/v4/utils"
	"github.com/roadrunner-server/sdk/v4/worker"
)

// Pool managed set of inner worker processes.
type Pool interface {
	// GetConfig returns pool configuration.
	GetConfig() *pool.Config
	// Workers returns worker list associated with the pool.
	Workers() (workers []*worker.Process)
	// RemoveWorker removes worker from the pool.
	RemoveWorker(worker *worker.Process) error
	// Exec payload
	Exec(ctx context.Context, p *payload.Payload) (*payload.Payload, error)
	// Reset kill all workers inside the watcher and replaces with new
	Reset(ctx context.Context) error
	// Destroy all underlying stack (but let them to complete the task).
	Destroy(ctx context.Context)
}

const (
	// PluginName for the server
	PluginName string = "server"
	// RPCPluginName is the name of the RPC plugin, should be in sync with rpc/config.go
	RPCPluginName string = "rpc"
	// RrRelay env variable key (internal)
	RrRelay string = "RR_RELAY"
	// RrRPC env variable key (internal) if the RPC presents
	RrRPC string = "RR_RPC"
	// RrVersion env variable
	RrVersion string = "RR_VERSION"

	// internal
	delim string = "://"
	unix  string = "unix"
	tcp   string = "tcp"
	pipes string = "pipes"
)

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if config section exists.
	Has(name string) bool
	// RRVersion is the roadrunner current version
	RRVersion() string
}

type NamedLogger interface {
	NamedLogger(name string) *zap.Logger
}

// Plugin manages worker
type Plugin struct {
	mu sync.Mutex

	cfg          *Config
	rpcCfg       *RPCConfig
	preparedCmd  []string
	preparedEnvs []string

	log     *zap.Logger
	factory pool.Factory

	pools []Pool
}

// Init application provider.
func (p *Plugin) Init(cfg Configurer, log NamedLogger) error {
	const op = errors.Op("server_plugin_init")
	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	err := cfg.UnmarshalKey(PluginName, &p.cfg)
	if err != nil {
		return errors.E(op, errors.Init, err)
	}

	err = cfg.UnmarshalKey(RPCPluginName, &p.rpcCfg)
	if err != nil {
		return errors.E(op, errors.Init, err)
	}

	err = p.cfg.InitDefaults()
	if err != nil {
		return errors.E(op, errors.Init, err)
	}

	p.log = new(zap.Logger)
	p.log = log.NamedLogger(PluginName)
	p.preparedCmd = append(p.preparedCmd, strings.Split(p.cfg.Command, " ")...)

	p.preparedEnvs = append(os.Environ(), fmt.Sprintf(RrRelay+"=%s", p.cfg.Relay))
	if p.rpcCfg != nil && p.rpcCfg.Listen != "" {
		p.preparedEnvs = append(p.preparedEnvs, fmt.Sprintf("%s=%s", RrRPC, p.rpcCfg.Listen))
	}

	// set env variables from the config
	if len(p.cfg.Env) > 0 {
		for k, v := range p.cfg.Env {
			p.preparedEnvs = append(p.preparedEnvs, fmt.Sprintf("%s=%s", strings.ToUpper(k), os.Expand(v, os.Getenv)))
		}
	}

	p.preparedEnvs = append(p.preparedEnvs, fmt.Sprintf("%s=%s", RrVersion, cfg.RRVersion()))

	p.pools = make([]Pool, 0, 4)

	p.factory, err = initFactory(p.log, p.cfg.Relay, p.cfg.RelayTimeout)
	if err != nil {
		return errors.E(op, err)
	}

	return nil
}

// Name contains service name.
func (p *Plugin) Name() string {
	return PluginName
}

// Serve (Start) server plugin (just a mock here to satisfy interface)
func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)

	if p.cfg.OnInit != nil {
		err := p.runOnInitCommand()
		if err != nil {
			p.log.Error("on_init was finished with errors", zap.Error(err))
		}
	}

	return errCh
}

// Stop used to close chosen in config factory
func (p *Plugin) Stop(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// destroy all pools
	for i := 0; i < len(p.pools); i++ {
		if p.pools[i] != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*p.pools[i].GetConfig().DestroyTimeout)
			p.pools[i].Destroy(ctx)
			cancel()
		}
	}

	// just to be sure, that all logs are synced
	time.Sleep(time.Second)
	return p.factory.Close()
}

// CmdFactory provides worker command factory associated with given context
func (p *Plugin) CmdFactory(env map[string]string) func() *exec.Cmd {
	return func() *exec.Cmd {
		var cmd *exec.Cmd

		if len(p.preparedCmd) == 1 {
			cmd = exec.Command(p.preparedCmd[0]) //nolint:gosec
		} else {
			cmd = exec.Command(p.preparedCmd[0], p.preparedCmd[1:]...) //nolint:gosec
		}

		// copy prepared envs
		cmd.Env = make([]string, len(p.preparedEnvs))
		copy(cmd.Env, p.preparedEnvs)

		// append external envs
		if len(env) > 0 {
			for k, v := range env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
			}
		}

		utils.IsolateProcess(cmd)
		// if user is not empty, and OS is linux or macos
		// execute php worker from that particular user
		if p.cfg.User != "" {
			err := utils.ExecuteFromUser(cmd, p.cfg.User)
			if err != nil {
				return nil
			}
		}

		return cmd
	}
}

// UID returns a user id (if specified by user)
func (p *Plugin) UID() int {
	if p.cfg.User == "" {
		return 0
	}

	usr, err := user.Lookup(p.cfg.User)
	if err != nil {
		p.log.Error("failed to get user", zap.String("id", p.cfg.User))
		return 0
	}

	usrI32, err := strconv.ParseInt(usr.Uid, 10, 32)
	if err != nil {
		p.log.Error("failed to parse user id", zap.String("id", p.cfg.User))
		return 0
	}

	return int(usrI32)
}

// GID returns a group id (if specified by user)
func (p *Plugin) GID() int {
	if p.cfg.User == "" {
		return 0
	}

	usr, err := user.Lookup(p.cfg.User)
	if err != nil {
		p.log.Error("failed to get user", zap.String("id", p.cfg.User))
		return 0
	}

	grI32, err := strconv.ParseInt(usr.Gid, 10, 32)
	if err != nil {
		p.log.Error("failed to parse group id", zap.String("id", p.cfg.Group))
		return 0
	}

	return int(grI32)
}

// customCmd used as and enhancement for the CmdFactory to use with a custom command string (used by default)
func (p *Plugin) customCmd(env map[string]string) func(command string) *exec.Cmd {
	return func(command string) *exec.Cmd {
		// if no command provided, use the server's one
		if command == "" {
			command = p.cfg.Command
		}

		var cmd *exec.Cmd

		preparedCmd := make([]string, 0, 10)
		preparedCmd = append(preparedCmd, strings.Split(command, " ")...)

		if len(preparedCmd) == 1 {
			cmd = exec.Command(preparedCmd[0]) //nolint:gosec
		} else {
			cmd = exec.Command(preparedCmd[0], preparedCmd[1:]...) //nolint:gosec
		}

		// copy prepared envs
		cmd.Env = make([]string, len(p.preparedEnvs))
		copy(cmd.Env, p.preparedEnvs)

		// append external envs
		if len(env) > 0 {
			for k, v := range env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
			}
		}

		utils.IsolateProcess(cmd)
		// if user is not empty, and OS is linux or macos
		// execute php worker from that particular user
		if p.cfg.User != "" {
			err := utils.ExecuteFromUser(cmd, p.cfg.User)
			if err != nil {
				p.log.Panic("can't execute command from the user", zap.String("user", p.cfg.User), zap.Error(err))
				return nil
			}
		}

		return cmd
	}
}

// NewWorker issues new standalone worker.
func (p *Plugin) NewWorker(ctx context.Context, env map[string]string) (*worker.Process, error) {
	const op = errors.Op("server_plugin_new_worker")

	spawnCmd := p.CmdFactory(env)

	w, err := p.factory.SpawnWorkerWithTimeout(ctx, spawnCmd())
	if err != nil {
		return nil, errors.E(op, err)
	}

	return w, nil
}

// NewPool issues new worker pool.
func (p *Plugin) NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *zap.Logger) (*staticPool.Pool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pl, err := staticPool.NewPool(ctx, p.customCmd(env), p.factory, cfg, p.log)
	if err != nil {
		return nil, err
	}

	p.pools = append(p.pools, pl)

	return pl, nil
}

// creates relay and worker factory.
func initFactory(log *zap.Logger, relay string, timeout time.Duration) (pool.Factory, error) {
	const op = errors.Op("server_plugin_init_factory")
	if relay == "" || relay == pipes {
		return pipe.NewPipeFactory(log), nil
	}

	dsn := strings.Split(relay, delim)
	if len(dsn) != 2 {
		return nil, errors.E(op, errors.Network, errors.Str("invalid DSN (tcp://:6001, unix://file.sock)"))
	}

	lsn, err := utils.CreateListener(relay)
	if err != nil {
		return nil, errors.E(op, errors.Network, err)
	}

	switch dsn[0] {
	// sockets group
	case unix:
		return socket.NewSocketServer(lsn, timeout, log), nil
	case tcp:
		return socket.NewSocketServer(lsn, timeout, log), nil
	default:
		return nil, errors.E(op, errors.Network, errors.Str("invalid DSN (tcp://:6001, unix://file.sock)"))
	}
}
