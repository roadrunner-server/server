package server

import (
	"context"
	"log/slog"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"github.com/roadrunner-server/errors"

	"github.com/roadrunner-server/pool/v2/pool"
	staticPool "github.com/roadrunner-server/pool/v2/pool/static_pool"
	"github.com/roadrunner-server/pool/v2/worker"
)

// Plugin manages worker
type Plugin struct {
	mu sync.Mutex

	cfg          *Config
	rpcCfg       *RPCConfig
	preparedCmd  []string
	preparedEnvs []string

	uid int
	gid int

	log     *slog.Logger
	factory pool.Factory
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

	p.log = log.NamedLogger(PluginName)

	// resolve the configured run-as user's uid/gid once
	if p.cfg.User != "" {
		usr, err := user.Lookup(p.cfg.User)
		if err != nil {
			return errors.E(op, errors.Init, err)
		}

		p.uid, err = strconv.Atoi(usr.Uid)
		if err != nil {
			return errors.E(op, errors.Init, err)
		}

		p.gid, err = strconv.Atoi(usr.Gid)
		if err != nil {
			return errors.E(op, errors.Init, err)
		}
	}

	p.preparedCmd = prepareCmd(p.cfg.Command)

	p.preparedEnvs = append(os.Environ(), RrRelay+"="+p.cfg.Relay)
	if p.rpcCfg != nil && p.rpcCfg.Listen != "" {
		p.preparedEnvs = append(p.preparedEnvs, RrRPC+"="+p.rpcCfg.Listen)
	}

	// set env variables from the config
	for k, v := range p.cfg.Env {
		p.preparedEnvs = append(p.preparedEnvs, strings.ToUpper(k)+"="+os.ExpandEnv(v))
	}

	p.preparedEnvs = append(p.preparedEnvs, RrVersion+"="+cfg.RRVersion())

	p.factory, err = initFactory(p.log, p.cfg.Relay)
	if err != nil {
		return errors.E(op, err)
	}

	return nil
}

// Name contains the service name.
func (p *Plugin) Name() string {
	return PluginName
}

// Serve (Start) server plugin (just a mock here to satisfy interface)
func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 1)

	if p.cfg.OnInit != nil {
		err := newCommand(p.log, p.cfg.OnInit).start()
		if err != nil {
			p.log.Error("on_init was finished with errors", "error", err)
			// if exit_on_error is set, we should return error and stop the server
			if p.cfg.OnInit.ExitOnError {
				errCh <- err
				return errCh
			}
		}
	}

	return errCh
}

// Stop used to stop all allocated pools
func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.factory.Close()
}

// NewWorker issues new standalone worker.
func (p *Plugin) NewWorker(ctx context.Context, env map[string]string) (*worker.Process, error) {
	const op = errors.Op("server_plugin_new_worker")

	spawnCmd := p.cmdFactory(env)

	w, err := p.factory.SpawnWorkerWithContext(ctx, spawnCmd())
	if err != nil {
		return nil, errors.E(op, err)
	}

	return w, nil
}

// NewPool issues new worker pool.
func (p *Plugin) NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *slog.Logger) (*staticPool.Pool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pl, err := staticPool.NewPool(ctx, pool.Command(p.customCmd(env)), p.factory, cfg, p.log, staticPool.WithQueueSize(cfg.MaxQueueSize))
	if err != nil {
		return nil, err
	}

	return pl, nil
}

func (p *Plugin) NewPoolWithOptions(ctx context.Context, cfg *pool.Config, env map[string]string, _ *slog.Logger, options ...staticPool.Options) (*staticPool.Pool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pl, err := staticPool.NewPool(ctx, pool.Command(p.customCmd(env)), p.factory, cfg, p.log, options...)
	if err != nil {
		return nil, err
	}

	return pl, nil
}

// UID returns a user id (if specified by user)
func (p *Plugin) UID() int {
	return p.uid
}

// GID returns a group id (if specified by user)
func (p *Plugin) GID() int {
	return p.gid
}
