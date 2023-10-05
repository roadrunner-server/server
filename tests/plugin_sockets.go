package tests

import (
	"context"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v4/payload"
	serverImpl "github.com/roadrunner-server/server/v4"
)

type Foo2 struct {
	configProvider Configurer
	wf             Server
	pool           Pool
}

func (f *Foo2) Init(p Configurer, workerFactory Server) error {
	f.configProvider = p
	f.wf = workerFactory
	return nil
}

func (f *Foo2) Serve() chan error {
	const op = errors.Op("serve")
	var err error
	errCh := make(chan error, 1)
	conf := &serverImpl.Config{}

	// test payload for echo
	r := &payload.Payload{
		Context: nil,
		Body:    []byte(Response),
	}

	err = f.configProvider.UnmarshalKey(ConfigSection, conf)
	if err != nil {
		errCh <- err
		return errCh
	}

	// test CMDFactory
	cmd := f.wf.CmdFactory(nil)
	if cmd == nil {
		errCh <- errors.E(op, "command is nil")
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
	rs, err := f.pool.Exec(context.Background(), r, make(chan struct{}))
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

func (f *Foo2) Stop(context.Context) error {
	return nil
}
