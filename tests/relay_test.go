package tests

import (
	"testing"

	"tests/helpers"

	"github.com/roadrunner-server/server/v6"
)

// The fixture plugins below drive the actual assertions: each one asks the
// server plugin for a worker and a pool, execs an echo payload through both and
// pushes anything unexpected onto the container's error channel. Start turns an
// error there into a test failure, so booting the container is the assertion.

// TestPipesRelay covers the default relay, where the worker talks over stdin
// and stdout.
func TestPipesRelay(t *testing.T) {
	helpers.Start(t, "configs/.rr.yaml", []any{&server.Plugin{}, &Foo{}})
}

// TestPipesRelayBigResponse covers a response larger than a single frame, which
// exercises the relay's chunking.
func TestPipesRelayBigResponse(t *testing.T) {
	helpers.Start(t, "configs/.rr-pipes-big-resp.yaml", []any{&server.Plugin{}, &Foo4{}})
}

// TestSocketsRelay covers the unix socket relay.
func TestSocketsRelay(t *testing.T) {
	helpers.Start(t, "configs/.rr-sockets.yaml", []any{&server.Plugin{}, &Foo2{}})
}

// TestTCPRelay covers the tcp relay.
func TestTCPRelay(t *testing.T) {
	helpers.Start(t, "configs/.rr-tcp.yaml", []any{&server.Plugin{}, &Foo3{}})
}

// TestPoolWithOptions covers NewPool called with explicit options rather than
// the config defaults.
func TestPoolWithOptions(t *testing.T) {
	helpers.Start(t, "configs/.rr-tcp.yaml", []any{&server.Plugin{}, &Foo5{}})
}

// TestServerEnvReachesWorker proves the server.env block is passed to the
// worker process, including a value built from an OS variable through ${...}
// expansion. Both env configs in this directory were unreferenced before.
func TestServerEnvReachesWorker(t *testing.T) {
	t.Setenv("RR_TEST_FROM_OS", "from-os")

	helpers.Start(t, "configs/.rr-env.yaml", []any{&server.Plugin{}, &FooEnv{}})
}
