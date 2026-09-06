//go:build linux || darwin || freebsd

package server

import (
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestInitRelaySocketPermissions(t *testing.T) {
	// Use a short path to fit the UNIX socket address limit.
	dir, err := os.MkdirTemp("", "rr-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })

	tests := []struct {
		mode string
		want os.FileMode
	}{
		{mode: "0660", want: 0o660},
		{mode: "0000", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			path := filepath.Join(dir, tt.mode+".sock")
			relay := "unix://" + path
			v := viper.New()
			v.Set("server.command", "php worker.php")
			v.Set("server.relay", relay)
			v.Set("server.relay_socket", map[string]any{
				"mode": tt.mode,
				"uid":  os.Getuid(),
				"gid":  os.Getgid(),
			})
			v.Set("server.env", map[string]string{"socket_test": "value"})
			v.Set("rpc.listen", "tcp://127.0.0.1:6001")
			cfg, err := InitMockCfg(v)
			require.NoError(t, err)

			p := &Plugin{}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			require.NoError(t, p.Init(cfg, NewTestLogger(log)))
			closeFactory := sync.OnceValue(p.factory.Close)
			t.Cleanup(func() { require.NoError(t, closeFactory()) })

			info, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, os.ModeSocket, info.Mode()&os.ModeSocket)
			require.Equal(t, tt.want, info.Mode().Perm())
			stat, ok := info.Sys().(*syscall.Stat_t)
			require.True(t, ok)
			require.EqualValues(t, os.Getuid(), stat.Uid)
			require.EqualValues(t, os.Getgid(), stat.Gid)
			require.Nil(t, p.ids)
			require.Empty(t, p.cfg.User)
			require.Empty(t, p.cfg.Group)
			require.Contains(t, p.preparedEnvs, RrRelay+"="+relay)
			require.Contains(t, p.preparedEnvs, RrRPC+"=tcp://127.0.0.1:6001")
			require.Contains(t, p.preparedEnvs, "SOCKET_TEST=value")

			require.NoError(t, closeFactory())
			_, err = os.Lstat(path)
			require.ErrorIs(t, err, os.ErrNotExist)
			dialer := net.Dialer{Timeout: time.Second}
			conn, err := dialer.DialContext(t.Context(), "unix", path)
			if conn != nil {
				require.NoError(t, conn.Close())
			}
			require.Error(t, err)
		})
	}
}

func TestInitRelaySocketBeforeFilesystemEffects(t *testing.T) {
	// Use a short path to fit the UNIX socket address limit.
	dir, err := os.MkdirTemp("", "rr-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })

	path := filepath.Join(dir, "existing.sock")
	lc := net.ListenConfig{}
	listener, err := lc.Listen(t.Context(), "unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	before, err := os.Lstat(path)
	require.NoError(t, err)

	for _, name := range []string{"new.sock", "existing.sock"} {
		t.Run(name, func(t *testing.T) {
			v := viper.New()
			v.Set("server.command", "php worker.php")
			v.Set("server.relay", "unix://"+filepath.Join(dir, name))
			v.Set("server.relay_socket.mode", "0888")
			cfg, err := InitMockCfg(v)
			require.NoError(t, err)

			p := &Plugin{}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			require.ErrorContains(t, p.Init(cfg, NewTestLogger(log)), "server.relay_socket")
			require.Nil(t, p.factory)
			require.Nil(t, p.preparedEnvs)

			_, err = os.Lstat(filepath.Join(dir, "new.sock"))
			require.ErrorIs(t, err, os.ErrNotExist)
			after, err := os.Lstat(path)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after))
			require.Equal(t, before.Mode(), after.Mode())
		})
	}
}
