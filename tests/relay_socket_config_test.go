package tests

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	mocklogger "tests/mock"

	"github.com/roadrunner-server/config/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
)

func TestRelaySocketFileInvalid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("UNIX socket attributes are not supported on Windows.")
	}

	for _, tc := range []struct {
		name    string
		relay   string
		options string
		wantErr string
	}{
		{name: "default pipes", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "explicit pipes", relay: "pipes", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "TCP options", relay: "tcp://127.0.0.1:0", options: `{mode: "0600"}`, wantErr: "filesystem unix:// address"},
		{name: "unquoted mode", relay: "unix://relay.sock", options: "{mode: 0660}", wantErr: "invalid unix socket mode"},
		{name: "scalar options", relay: "unix://relay.sock", options: "false", wantErr: "expected a map"},
		{name: "negative UID", relay: "unix://relay.sock", options: "{uid: -1}", wantErr: "invalid unix socket uid"},
		{name: "negative GID", relay: "unix://relay.sock", options: "{gid: -1}", wantErr: "invalid unix socket gid"},
		{name: "reserved UID", relay: "unix://relay.sock", options: "{uid: 4294967295}", wantErr: "invalid unix socket uid"},
		{name: "reserved GID", relay: "unix://relay.sock", options: "{gid: 4294967295}", wantErr: "invalid unix socket gid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			cfg := relaySocketConfigFile(t, fmt.Sprintf(`version: "3"
server:
  command: php worker.php
  user: rr-definitely-missing-user
  relay: %q
  relay_socket: %s
`, tc.relay, tc.options))
			p := &server.Plugin{}
			log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
			err := p.Init(cfg, log)
			if err == nil {
				require.NoError(t, p.Stop(t.Context()))
			}
			require.ErrorContains(t, err, "relay_socket")
			require.ErrorContains(t, err, tc.wantErr)
			_, err = os.Lstat("relay.sock")
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func relaySocketConfigFile(t *testing.T, contents string) *config.Plugin {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".rr.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg := &config.Plugin{Path: path}
	require.NoError(t, cfg.Init())
	return cfg
}
