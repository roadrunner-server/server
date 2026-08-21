package tests

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"tests/helpers"

	"github.com/roadrunner-server/metrics/v6"
	rpcPlugin "github.com/roadrunner-server/rpc/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
)

const metricsAddr = "127.0.0.1:9254"

// TestTCPRelayWithOnInit runs the on_init command before the pool starts and
// then exercises the pool over the tcp relay.
func TestTCPRelayWithOnInit(t *testing.T) {
	helpers.Start(t, "configs/.rr-tcp-on-init.yaml", []any{&server.Plugin{}, &Foo2{}})
}

// TestSocketsRelayWithOnInit is the same over the socket relay.
func TestSocketsRelayWithOnInit(t *testing.T) {
	helpers.Start(t, "configs/.rr-sockets-on-init.yaml", []any{&server.Plugin{}, &Foo2{}})
}

// TestOnInitFastClose covers an on_init command that exits immediately: the
// pool must still come up rather than treating the early exit as a failure.
func TestOnInitFastClose(t *testing.T) {
	helpers.Start(t, "configs/.rr-sockets-on-init-fast-close.yaml", []any{&server.Plugin{}, &Foo2{}})
}

// TestOnInitErrorFailsServe covers an on_init command that exits non-zero.
func TestOnInitErrorFailsServe(t *testing.T) {
	_ = helpers.StartExpectServeError(t, "configs/.rr-on-init-error.yaml", []any{&server.Plugin{}},
		helpers.WithConfigVersion("v2024.1.0"))
}

// TestOnInitTimeoutFailsServe covers an on_init command that never returns; the
// error has to name the timeout so the cause is obvious from CI output.
func TestOnInitTimeoutFailsServe(t *testing.T) {
	err := helpers.StartExpectServeError(t, "configs/.rr-on-init-error-timeout.yaml", []any{&server.Plugin{}},
		helpers.WithConfigVersion("v2024.1.0"))

	require.ErrorContains(t, err, "startup process has been killed by timeout")
}

// TestOnInitRunsWithMetricsEndpoint boots the config whose on_init script
// declares a foo_bar_test counter over rpc and checks the collector reaches
// the exporter.
func TestOnInitRunsWithMetricsEndpoint(t *testing.T) {
	helpers.Start(t,
		"configs/.rr-metrics-oninit.yaml",
		[]any{&server.Plugin{}, &rpcPlugin.Plugin{}, &metrics.Plugin{}},
		helpers.WithTCPProbe(metricsAddr),
	)

	// the script runs while the server is coming up, so the collector may
	// land shortly after the exporter port opens
	require.Eventually(t, func() bool {
		body, err := scrapeMetrics(t)
		return err == nil && strings.Contains(body, "foo_bar_test")
	}, 10*time.Second, 100*time.Millisecond,
		"the on_init script's foo_bar_test collector never showed up in the exporter output")
}

// scrapeMetrics fetches the exporter output.
func scrapeMetrics(t *testing.T) (string, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+metricsAddr+"/metrics", nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from the metrics endpoint", resp.StatusCode)
	}

	return string(body), nil
}
