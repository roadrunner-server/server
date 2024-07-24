package server

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/roadrunner-server/errors"
	"go.uber.org/zap"

	"github.com/roadrunner-server/pool/pool"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	"github.com/roadrunner-server/pool/worker"
)

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

	// here we may have 2 cases: command declared as a space-separated string or as a slice
	switch len(p.cfg.Command) {
	// command defined as a space-separated string
	case 1:
		// we know that the len is 1, so we can safely use the first element
		p.preparedCmd = append(p.preparedCmd, strings.Split(p.cfg.Command[0], " ")...)
	default:
		// we have a slice with a 2 or more elements
		// first element is the command, the rest are arguments
		p.preparedCmd = p.cfg.Command
	}

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
			p.log.Error("on_init was finished with errors", zap.Error(err))
		}
	}

	return errCh
}

// Stop used to stop all allocated pools
func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	wg := &sync.WaitGroup{}
	wg.Add(len(p.pools))

	// destroy all pools in parallel
	for i := 0; i < len(p.pools); i++ {
		go func(idx int) {
			if p.pools[idx] == nil || p.pools[idx].(*staticPool.Pool) == nil {
				wg.Done()
				return
			}
			ctx2, cancel := context.WithTimeout(ctx, p.pools[idx].GetConfig().DestroyTimeout)
			p.pools[idx].Destroy(ctx2)
			cancel()
			wg.Done()
		}(i)
	}

	wg.Wait()
	// just to be sure that all logs are synced
	time.Sleep(time.Second)
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
func (p *Plugin) NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *zap.Logger) (*staticPool.Pool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pl, err := staticPool.NewPool(ctx, pool.Command(p.customCmd(env)), p.factory, cfg, p.log, staticPool.WithQueueSize(cfg.MaxQueueSize))
	if err != nil {
		return nil, err
	}

	p.pools = append(p.pools, pl)

	return pl, nil
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
