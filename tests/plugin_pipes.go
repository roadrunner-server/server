package tests

import (
	"context"
	"os/exec"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/payload"
	"github.com/roadrunner-server/pool/pool"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	"github.com/roadrunner-server/pool/worker"
	"github.com/roadrunner-server/server/v5"
	"go.uber.org/zap"
)

const ConfigSection = "server"
const Response = "test"

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if config section exists.
	Has(name string) bool
}

// Server creates workers for the application.
type Server interface {
	CmdFactory(env map[string]string) func() *exec.Cmd
	NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, _ *zap.Logger) (*staticPool.Pool, error)
	NewWorker(ctx context.Context, env map[string]string) (*worker.Process, error)
}

type Pool interface {
	// Workers returns worker list associated with the pool.
	Workers() (workers []*worker.Process)
	// Exec payload
	Exec(ctx context.Context, p *payload.Payload, stopCh chan struct{}) (chan *staticPool.PExec, error)
	// Reset kill all workers inside the watcher and replaces with new
	Reset(ctx context.Context) error
	// Destroy all underlying stack (but let them complete the task).
	Destroy(ctx context.Context)
}

var testPoolConfig = &pool.Config{ //nolint:gochecknoglobals
	NumWorkers:      10,
	MaxJobs:         100,
	AllocateTimeout: time.Second * 10,
	DestroyTimeout:  time.Second * 10,
	Supervisor: &pool.SupervisorConfig{
		WatchTick:       60 * time.Second,
		TTL:             1000 * time.Second,
		IdleTTL:         10 * time.Second,
		ExecTTL:         10 * time.Second,
		MaxWorkerMemory: 1000,
	},
}

type Foo struct {
	configProvider Configurer
	wf             Server
	pool           Pool
}

func (f *Foo) Init(p Configurer, workerFactory Server) error {
	f.configProvider = p
	f.wf = workerFactory
	return nil
}

func (f *Foo) Serve() chan error {
	const op = errors.Op("serve")

	// test payload for echo
	r := &payload.Payload{
		Context: nil,
		Body:    []byte(Response),
	}

	errCh := make(chan error, 1)

	conf := &server.Config{}
	var err error
	err = f.configProvider.UnmarshalKey(ConfigSection, conf)
	if err != nil {
		errCh <- err
		return errCh
	}

	// test CMDFactory
	cmd := f.wf.CmdFactory(nil)
	if cmd == nil {
		errCh <- errors.E(op, errors.Str("command is nil"))
		return errCh
	}

	// test worker creation
	w, err := f.wf.NewWorker(context.Background(), nil)
	if err != nil {
		errCh <- err
		return errCh
	}

	go func() {
		_ = w.Wait()
	}()

	// test that our worker is functional

	rsp, err := w.Exec(context.Background(), r)
	if err != nil {
		errCh <- err
		return errCh
	}

	if string(rsp.Body) != Response {
		errCh <- errors.E("response from worker is wrong", errors.Errorf("response: %s", rsp.Body))
		return errCh
	}

	// should not be errors
	err = w.Stop()
	if err != nil {
		errCh <- err
		return errCh
	}

	// test pool
	f.pool, err = f.wf.NewPool(context.Background(), testPoolConfig, nil, nil)
	if err != nil {
		errCh <- err
		return errCh
	}

	// test pool execution
	rs, err := f.pool.Exec(context.Background(), r, make(chan struct{}, 1))
	if err != nil {
		errCh <- err
		return errCh
	}

	rspp := <-rs

	// echo of the "test" should be -> test
	if string(rspp.Body()) != Response {
		errCh <- errors.E("response from worker is wrong", errors.Errorf("response: %s", rspp.Body()))
		return errCh
	}

	return errCh
}

func (f *Foo) Stop(context.Context) error {
	return nil
}
