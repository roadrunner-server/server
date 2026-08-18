package tests

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/server/v6"
)

// The cases below all describe a broken configuration. Each must be rejected at
// the boot stage where the problem is detectable, rather than starting a server
// that cannot work.

// TestMissingConfigFileFailsInit covers a config path that does not exist.
func TestMissingConfigFileFailsInit(t *testing.T) {
	_ = helpers.StartExpectInitError(t, "configs/.rrrrrrrrrr.yaml", []any{&server.Plugin{}},
		helpers.WithConfigVersion("v2024.1.0"))
}

// TestUnknownRelayFailsInit covers a relay value the plugin does not implement.
func TestUnknownRelayFailsInit(t *testing.T) {
	_ = helpers.StartExpectInitError(t, "configs/.rr-wrong-relay.yaml", []any{&server.Plugin{}},
		helpers.WithConfigVersion("v2024.1.0"))
}

// TestUnrunnableCommandFailsServe covers a command that cannot be executed: the
// config is well formed, so this only shows up when workers are allocated.
func TestUnrunnableCommandFailsServe(t *testing.T) {
	_ = helpers.StartExpectServeError(t, "configs/.rr-wrong-command.yaml", []any{&server.Plugin{}, &Foo{}},
		helpers.WithConfigVersion("v2024.1.0"))
}

// TestUnrunnableOnInitCommandFailsServe is the same for the on_init command.
// The fixture has to be registered as well: the server plugin only reports the
// failure once something asks it for a pool.
func TestUnrunnableOnInitCommandFailsServe(t *testing.T) {
	_ = helpers.StartExpectServeError(t, "configs/.rr-wrong-command-on-init.yaml", []any{&server.Plugin{}, &Foo3{}},
		helpers.WithConfigVersion("v2024.1.0"))
}

// TestWorkerExceptionFailsServe covers a worker script that throws during
// startup, so the pool cannot be filled.
func TestWorkerExceptionFailsServe(t *testing.T) {
	_ = helpers.StartExpectServeError(t, "configs/.rr-script-err.yaml", []any{&server.Plugin{}, &Foo{}},
		helpers.WithConfigVersion("v2024.1.0"))
}
