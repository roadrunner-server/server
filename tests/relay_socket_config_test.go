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

func TestRelaySocketFileEmptyBlock(t *testing.T) {
	for _, relay := range []string{"", "pipes", "tcp://127.0.0.1:0"} {
		t.Run(relay, func(t *testing.T) {
			cfg := relaySocketConfigFile(t, "yaml", fmt.Sprintf("version: \"3\"\nserver:\n  command: php worker.php\n  relay: %q\n  relay_socket: {}\n", relay))
			require.True(t, cfg.Has("server.relay_socket"))

			p := &server.Plugin{}
			log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
			err := p.Init(cfg, log)
			if err == nil {
				require.NoError(t, p.Stop(t.Context()))
			}
			require.ErrorContains(t, err, "server.relay_socket")
			require.NotContains(t, err.Error(), "server_plugin_init_factory")
		})
	}
}

func TestRelaySocketFileInvalidOwnership(t *testing.T) {
	t.Setenv("RR_SERVER_SOCKET_UNSET", "")
	require.NoError(t, os.Unsetenv("RR_SERVER_SOCKET_UNSET"))

	tests := []struct {
		name  string
		value string
	}{
		{name: "false", value: "false"},
		{name: "true", value: "true"},
		{name: "fraction", value: "1.9"},
		{name: "negative fraction", value: "-0.5"},
		{name: "empty string", value: `""`},
		{name: "unset environment", value: `"${RR_SERVER_SOCKET_UNSET}"`},
		{name: "negative integer", value: "-1"},
		{name: "integer overflow", value: "4294967295"},
		{name: "float overflow", value: "4294967295.0"},
		{name: "not a number", value: ".nan"},
		{name: "infinity", value: ".inf"},
		{name: "negative infinity", value: "-.inf"},
		{name: "decimal string", value: `"33.0"`},
		{name: "whitespace", value: `" "`},
		{name: "sequence", value: "[0]"},
		{name: "map", value: "{}"},
	}

	for _, field := range []string{"uid", "gid"} {
		for _, tt := range tests {
			t.Run(field+"/"+tt.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "relay.sock")
				cfg := relaySocketConfigFile(t, "yaml", fmt.Sprintf("version: \"3\"\nserver:\n  command: php worker.php\n  relay: %q\n  relay_socket:\n    %s: %s\n", "unix://"+path, field, tt.value))
				p := &server.Plugin{}
				log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
				err := p.Init(cfg, log)
				if err == nil {
					require.NoError(t, p.Stop(t.Context()))
				}
				require.ErrorContains(t, err, "server.relay_socket."+field)
				require.NotContains(t, err.Error(), "server_plugin_init_factory")
				_, err = os.Lstat(path)
				require.ErrorIs(t, err, os.ErrNotExist)
			})
		}
	}
}

func TestRelaySocketFileValidOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("UNIX socket options are not supported on windows")
	}
	t.Setenv("RR_SERVER_SOCKET_ID", "0")
	zero, id := 0, 33
	tests := []struct {
		name   string
		format string
		value  string
		want   *int
	}{
		{name: "omitted IDs", format: "yaml"},
		{name: "nil IDs", format: "yaml", value: "    uid: null\n    gid: null\n"},
		{name: "numeric zero", format: "yaml", value: "    uid: 0\n    gid: 0\n", want: &zero},
		{name: "environment zero", format: "yaml", value: "    uid: \"${RR_SERVER_SOCKET_ID}\"\n    gid: \"${RR_SERVER_SOCKET_ID}\"\n", want: &zero},
		{name: "base zero strings", format: "yaml", value: "    uid: \"0x21\"\n    gid: \"041\"\n", want: &id},
		{name: "JSON integer floats", format: "json", value: `"uid": 33.0, "gid": 33.0`, want: &id},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A missing parent stops startup after validation without changing ownership.
			path := filepath.Join(t.TempDir(), "missing", "relay.sock")
			contents := fmt.Sprintf("version: \"3\"\nserver:\n  command: php worker.php\n  relay: %q\n  relay_socket:\n    mode: \"0660\"\n%s", "unix://"+path, tt.value)
			if tt.format == "json" {
				contents = fmt.Sprintf(`{"version":"3","server":{"command":"php worker.php","relay":%q,"relay_socket":{"mode":"0660",%s}}}`, "unix://"+path, tt.value)
			}
			cfg := relaySocketConfigFile(t, tt.format, contents)
			var decoded server.Config
			require.NoError(t, cfg.UnmarshalKey("server", &decoded))
			require.NotNil(t, decoded.RelaySocket)
			require.Equal(t, tt.want, decoded.RelaySocket.UID)
			require.Equal(t, tt.want, decoded.RelaySocket.GID)

			p := &server.Plugin{}
			log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
			require.ErrorContains(t, p.Init(cfg, log), "server_plugin_init_factory")
			_, err := os.Lstat(path)
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestRelaySocketFileNilOptions(t *testing.T) {
	for _, block := range []string{"", "  relay_socket: null\n"} {
		t.Run(block, func(t *testing.T) {
			cfg := relaySocketConfigFile(t, "yaml", "version: \"3\"\nserver:\n  command: php worker.php\n"+block)
			require.False(t, cfg.Has("server.relay_socket"))
			var decoded server.Config
			require.NoError(t, cfg.UnmarshalKey("server", &decoded))
			require.Nil(t, decoded.RelaySocket)

			p := &server.Plugin{}
			log := mocklogger.NewLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
			require.NoError(t, p.Init(cfg, log))
			t.Cleanup(func() { require.NoError(t, p.Stop(t.Context())) })
		})
	}
}

func relaySocketConfigFile(t *testing.T, format, contents string) *config.Plugin {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".rr."+format)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	cfg := &config.Plugin{Path: path}
	require.NoError(t, cfg.Init())
	return cfg
}
