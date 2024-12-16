package tests

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	mockLogger "tests/mock"

	"github.com/roadrunner-server/config/v5"
	"github.com/roadrunner-server/endure/v2"
	httpPlugin "github.com/roadrunner-server/http/v5"
	"github.com/roadrunner-server/logger/v5"
	"github.com/roadrunner-server/metrics/v5"
	"github.com/roadrunner-server/prometheus/v5"
	rpcPlugin "github.com/roadrunner-server/rpc/v5"
	"github.com/roadrunner-server/server/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAppPipes(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.2.0",
		Path:    "configs/.rr.yaml",
	}

	err := cont.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	stopCh <- struct{}{}
	wg.Wait()
}

func TestAppPipesBigResp(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.2.0",
		Path:    "configs/.rr-pipes-big-resp.yaml",
	}

	rd, wr, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = wr

	err = cont.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo4{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 5)
	stopCh <- struct{}{}
	wg.Wait()

	time.Sleep(time.Second)
	_ = wr.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, rd)
	require.NoError(t, err)
	require.GreaterOrEqual(t, strings.Count(buf.String(), "A"), 64000)
}

func TestAppSockets(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.2.0",
		Path:    "configs/.rr-sockets.yaml",
	}

	err := cont.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo2{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 5)
	stopCh <- struct{}{}
	wg.Wait()
}

func TestAppPipesException(t *testing.T) {
	container := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Path:    "configs/.rr-script-err.yaml",
		Version: "v2024.1.0",
	}

	err := container.RegisterAll(
		vp,
		&server.Plugin{},
		&logger.Plugin{},
		&httpPlugin.Plugin{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed on the message sent to STDOUT, see: https://docs.roadrunner.dev/error-codes/stdout-crc, invalid message: warning: some weird php error warning: some weird php error warning: some weird php error warning: some weird php error warning: some weird php error")
	_ = container.Stop()
}

func TestAppTCPOnInitError(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-on-init-error.yaml",
	}

	err := cont.Register(vp)
	require.NoError(t, err)

	err = cont.RegisterAll(
		&logger.Plugin{},
		&server.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	_, err = cont.Serve()
	require.Error(t, err)
}

func TestAppTCPOnInit(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-tcp-on-init.yaml",
	}

	err := cont.Register(vp)
	require.NoError(t, err)

	l, oLogger := mockLogger.ZapTestLogger(zap.DebugLevel)
	err = cont.RegisterAll(
		l,
		&server.Plugin{},
		&Foo2{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 10)
	stopCh <- struct{}{}
	wg.Wait()

	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 0").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 1").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 2").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 3").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 4").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 5").Len())
}

func TestAppSocketsOnInit(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-sockets-on-init.yaml",
	}

	err := cont.Register(vp)
	require.NoError(t, err)

	l, oLogger := mockLogger.ZapTestLogger(zap.DebugLevel)
	err = cont.RegisterAll(
		l,
		&server.Plugin{},
		&Foo2{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 10)
	stopCh <- struct{}{}
	wg.Wait()

	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 0\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 1\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 2\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 3\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 4\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 5\n").Len())
}

func TestAppSocketsOnInitFastClose(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-sockets-on-init-fast-close.yaml",
	}

	err := cont.Register(vp)
	require.NoError(t, err)

	l, oLogger := mockLogger.ZapTestLogger(zap.DebugLevel)
	err = cont.RegisterAll(
		l,
		&server.Plugin{},
		&Foo2{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 10)
	stopCh <- struct{}{}
	wg.Wait()

	require.Equal(t, 1, oLogger.FilterMessageSnippet("process wait").Len())
}

func TestAppTCP(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-tcp.yaml",
	}

	err := cont.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo3{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	stopCh <- struct{}{}
	wg.Wait()
}

func TestAppWrongConfig(t *testing.T) {
	container := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rrrrrrrrrr.yaml",
	}

	err := container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	require.Error(t, container.Init())
}

func TestAppWrongRelay(t *testing.T) {
	container := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-wrong-relay.yaml",
	}

	err := container.Register(vp)
	assert.NoError(t, err)

	err = container.Register(&server.Plugin{})
	assert.NoError(t, err)

	err = container.Register(&Foo3{})
	assert.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	assert.NoError(t, err)

	err = container.Init()
	assert.Error(t, err)

	_, err = container.Serve()
	assert.Error(t, err)

	_ = container.Stop()
}

func TestAppWrongCommand(t *testing.T) {
	container := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-wrong-command.yaml",
	}

	err := container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
}

func TestAppWrongCommandOnInit(t *testing.T) {
	container := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-wrong-command-on-init.yaml",
	}

	err := container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
}

func TestAppNoAppSectionInConfig(t *testing.T) {
	container := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-wrong-command.yaml",
	}

	err := container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
}

func TestOnInitMetrics(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-metrics-oninit.yaml",
	}

	err := cont.RegisterAll(
		vp,
		&server.Plugin{},
		&metrics.Plugin{},
		&prometheus.Plugin{},
		&rpcPlugin.Plugin{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	stopCh <- struct{}{}
	wg.Wait()
}

func TestNewPoolWithOptions(t *testing.T) {
	cont := endure.New(slog.LevelDebug)

	// config plugin
	vp := &config.Plugin{
		Version: "v2024.1.0",
		Path:    "configs/.rr-tcp.yaml",
	}

	err := cont.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo5{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	stopCh <- struct{}{}
	wg.Wait()
}
