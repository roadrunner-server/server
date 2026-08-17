package tests

import (
	"context"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/v2/payload"
)

// FooEnv asks the worker for the value of two environment variables declared in
// the server.env config block, proving they reached the worker process.
type FooEnv struct {
	wf Server
}

func (f *FooEnv) Init(_ Configurer, workerFactory Server) error {
	f.wf = workerFactory
	return nil
}

func (f *FooEnv) Serve() chan error {
	errCh := make(chan error, 1)

	pl, err := f.wf.NewPool(context.Background(), testPoolConfig, nil, nil)
	if err != nil {
		errCh <- err
		return errCh
	}

	for name, want := range map[string]string{
		"RR_PLAIN":    "plain-value",
		"RR_EXPANDED": "prefix-from-os-suffix",
	} {
		rs, errE := pl.Exec(context.Background(), &payload.Payload{Body: []byte(name)}, make(chan struct{}, 1))
		if errE != nil {
			errCh <- errE
			return errCh
		}

		if got := string((<-rs).Body()); got != want {
			errCh <- errors.Errorf("%s: want %q, got %q", name, want, got)
			return errCh
		}
	}

	return errCh
}

func (f *FooEnv) Stop(context.Context) error { return nil }

func (f *FooEnv) Name() string { return "foo_env" }
