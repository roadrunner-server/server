//go:build linux || darwin || freebsd

package tests

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	mocklogger "tests/mock"

	"github.com/roadrunner-server/pool/v2/payload"
	"github.com/roadrunner-server/pool/v2/pool"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
)

func TestRelaySocketRelays(t *testing.T) {
	worker, err := filepath.Abs("php_test_files/socket.php")
	require.NoError(t, err)
	t.Chdir(t.TempDir())
	t.Setenv("RR_SERVER_SOCKET_UID", strconv.Itoa(os.Getuid()))
	t.Setenv("RR_SERVER_SOCKET_GID", strconv.Itoa(os.Getgid()))

	// Measure the default permissions without changing the process umask.
	lc := net.ListenConfig{}
	listener, err := lc.Listen(t.Context(), "unix", "default.sock")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	info, err := os.Stat("default.sock")
	require.NoError(t, err)
	defaultMode := info.Mode().Perm()

	for _, tc := range []struct {
		name    string
		relay   string
		options string
		mode    os.FileMode
	}{
		{name: "default pipes"},
		{name: "pipes null options", relay: "pipes", options: "null"},
		{name: "pipes empty options", relay: "pipes", options: "{}"},
		{name: "TCP defaults", relay: "tcp://127.0.0.1:0"},
		{name: "TCP empty options", relay: "tcp://127.0.0.1:0", options: "{}"},
		{name: "UNIX defaults", relay: "unix://relay.sock", mode: defaultMode},
		{name: "UNIX empty options", relay: "unix://relay.sock", options: "{}", mode: defaultMode},
		{name: "UNIX mode only", relay: "unix://relay.sock", options: `{mode: "0600"}`, mode: 0o600},
		{name: "UNIX ownership without mode", relay: "unix://relay.sock", options: fmt.Sprintf("{uid: %d, gid: %d}", os.Getuid(), os.Getgid()), mode: defaultMode},
		{name: "UNIX environment ownership", relay: "unix://relay.sock", options: `{mode: "0640", uid: "${RR_SERVER_SOCKET_UID}", gid: "${RR_SERVER_SOCKET_GID}"}`, mode: 0o640},
		{name: "UNIX converted ownership", relay: "unix://relay.sock", options: fmt.Sprintf("{mode: \"0640\", uid: %d.5, gid: %d.5}", os.Getuid(), os.Getgid()), mode: 0o640},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			relay := tc.relay
			if strings.HasPrefix(relay, "tcp://") {
				reserved, errL := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
				require.NoError(t, errL)
				relay = "tcp://" + reserved.Addr().String()
				require.NoError(t, reserved.Close())
			}
			data := fmt.Sprintf(`version: "3"
server:
  command: [php, %q]
  relay: %q
`, worker, relay)
			if tc.options != "" {
				data += "  relay_socket: " + tc.options + "\n"
			}
			cfg := relaySocketConfigFile(t, data)
			p := &server.Plugin{}
			log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
			require.NoError(t, p.Init(cfg, log))
			stop := sync.OnceValue(func() error { return p.Stop(context.Background()) })
			t.Cleanup(func() { require.NoError(t, stop()) })

			network, address, socketRelay := strings.Cut(relay, "://")
			if network == "unix" {
				info, errS := os.Stat(address)
				require.NoError(t, errS)
				require.NotZero(t, info.Mode()&os.ModeSocket)
				require.Equal(t, tc.mode, info.Mode().Perm())
				stat := info.Sys().(*syscall.Stat_t)
				require.EqualValues(t, os.Getuid(), stat.Uid)
				require.EqualValues(t, os.Getgid(), stat.Gid)
			}

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			w, errW := p.NewWorker(ctx, nil)
			require.NoError(t, errW)
			wait := make(chan error, 1)
			go func() { wait <- w.Wait() }()
			stopWorker := sync.OnceFunc(func() {
				require.NoError(t, w.Stop())
				require.NoError(t, <-wait)
			})
			t.Cleanup(stopWorker)
			request := &payload.Payload{Body: []byte(tc.name)}
			response, errW := w.Exec(ctx, request)
			require.NoError(t, errW)
			require.Equal(t, request.Body, response.Body)
			stopWorker()

			wp, errP := p.NewPool(ctx, &pool.Config{NumWorkers: 1, AllocateTimeout: 5 * time.Second, DestroyTimeout: 5 * time.Second}, nil, nil)
			require.NoError(t, errP)
			destroy := sync.OnceFunc(func() { wp.Destroy(context.Background()) })
			t.Cleanup(destroy)
			responses, errP := wp.Exec(ctx, request, nil)
			require.NoError(t, errP)
			select {
			case result := <-responses:
				require.NotNil(t, result)
				require.NoError(t, result.Error())
				require.Equal(t, request.Body, result.Body())
			case <-ctx.Done():
				t.Fatal("timed out waiting for the pool response")
			}
			destroy()
			require.NoError(t, stop())
			if network == "unix" {
				_, errS := os.Lstat(address)
				require.ErrorIs(t, errS, os.ErrNotExist)
			}
			if socketRelay {
				dialer := net.Dialer{Timeout: time.Second}
				conn, errD := dialer.DialContext(t.Context(), network, address)
				if conn != nil {
					require.NoError(t, conn.Close())
				}
				require.Error(t, errD)
			}
		})
	}
}

func TestRelaySocketOwnershipError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Requires an unprivileged process.")
	}

	groups, err := os.Getgroups()
	require.NoError(t, err)
	otherGID := 0
	for otherGID == os.Getegid() || slices.Contains(groups, otherGID) {
		otherGID++
	}

	for _, tc := range []struct {
		name  string
		field string
		id    int
	}{
		{name: "root UID", field: "uid", id: 0},
		{name: "nonmember GID", field: "gid", id: otherGID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := relaySocketConfigFile(t, fmt.Sprintf(`version: "3"
server:
  command: php worker.php
  relay: unix://ownership.sock
  relay_socket: {%s: %d}
`, tc.field, tc.id))
			p := &server.Plugin{}
			log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
			err := p.Init(cfg, log)
			if err == nil {
				require.NoError(t, p.Stop(t.Context()))
			}
			require.ErrorContains(t, err, "chown unix socket")
			require.ErrorContains(t, err, syscall.EPERM.Error())
			_, err = os.Lstat("ownership.sock")
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}
