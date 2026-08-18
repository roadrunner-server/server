package helpers

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/endure/v2"
	"github.com/roadrunner-server/logger/v6"
	"github.com/stretchr/testify/require"
)

const (
	// defaultConfigVersion is the config schema version used by the test configs.
	defaultConfigVersion = "v2024.2.0"
	// probeTimeout caps how long Start waits for the rpc listener to answer.
	probeTimeout = time.Second * 15
	probeTick    = time.Millisecond * 20
	probeDial    = time.Second
)

// bootCfg holds the options applied to a container before it is started.
type bootCfg struct {
	version  string
	logLevel slog.Level
	probe    func(ctx context.Context) bool
}

// Option customizes the container built by Start.
type Option func(*bootCfg)

// WithConfigVersion overrides the config schema version.
func WithConfigVersion(v string) Option {
	return func(b *bootCfg) { b.version = v }
}

// WithLogLevel sets the endure container log level (debug by default).
func WithLogLevel(l slog.Level) Option {
	return func(b *bootCfg) { b.logLevel = l }
}

// WithTCPProbe makes Start return only once addr accepts a connection. The rpc
// listener binds after the storage drivers are constructed, so dialing it
// proves the plugin is ready to serve calls.
func WithTCPProbe(addr string) Option {
	return func(b *bootCfg) {
		b.probe = func(ctx context.Context) bool {
			d := net.Dialer{Timeout: probeDial}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return false
			}

			_ = conn.Close()
			return true
		}
	}
}

// Start registers the plugins, boots the container and waits for the probe, if
// any, to answer. Errors arriving on the container channel are reported through
// t.Errorf and stop the container, but they do not abort the test.
//
// The returned stop is idempotent and also registered with t.Cleanup.
func Start(t *testing.T, cfgPath string, plugins []any, opts ...Option) func() {
	t.Helper()

	cont, bc := newContainer(t, cfgPath, plugins, opts)
	require.NoError(t, cont.Init())

	ch, err := cont.Serve()
	require.NoError(t, err)

	stopCont := sync.OnceValue(cont.Stop)
	done := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case res := <-ch:
				if res == nil {
					return
				}
				t.Errorf("plugin %s reported an error: %v", res.VertexID, res.Error)
				if errS := stopCont(); errS != nil {
					t.Errorf("container stop: %v", errS)
				}
			case <-done:
				if errS := stopCont(); errS != nil {
					t.Errorf("container stop: %v", errS)
				}
				return
			}
		}
	})

	// The drain goroutine calls t.Errorf, so it has to be joined while the test
	// is still running.
	stop := sync.OnceFunc(func() {
		close(done)
		wg.Wait()
	})
	t.Cleanup(stop)

	if bc.probe != nil {
		require.Eventually(t, func() bool { return bc.probe(t.Context()) }, probeTimeout, probeTick, "rpc listener did not become ready")
	}

	return stop
}

// StartExpectInitError registers the plugins and requires Init to fail,
// returning its error.
func StartExpectInitError(t *testing.T, cfgPath string, plugins []any, opts ...Option) error {
	t.Helper()

	cont, _ := newContainer(t, cfgPath, plugins, opts)

	err := cont.Init()
	require.Error(t, err)

	return err
}

// StartExpectServeError registers the plugins, requires Init to pass and Serve
// to fail, and returns the Serve error.
func StartExpectServeError(t *testing.T, cfgPath string, plugins []any, opts ...Option) error {
	t.Helper()

	cont, _ := newContainer(t, cfgPath, plugins, opts)
	require.NoError(t, cont.Init())

	_, err := cont.Serve()
	require.Error(t, err)
	t.Cleanup(func() { _ = cont.Stop() })

	return err
}

// newContainer builds the container and registers the config, the logger and
// the caller's plugins. The container is not initialized yet.
func newContainer(t *testing.T, cfgPath string, plugins []any, opts []Option) (*endure.Endure, *bootCfg) {
	t.Helper()

	bc := &bootCfg{version: defaultConfigVersion, logLevel: slog.LevelDebug}
	for _, o := range opts {
		o(bc)
	}

	all := make([]any, 0, 2+len(plugins))
	all = append(all,
		&config.Plugin{Version: bc.version, Path: cfgPath},
		&logger.Plugin{},
	)

	cont := endure.New(bc.logLevel)
	require.NoError(t, cont.RegisterAll(append(all, plugins...)...))

	return cont, bc
}
