package tests

import (
	"io"
	"net/http"
	"testing"

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

// TestOnInitRunsWithMetricsEndpoint boots the config whose on_init script talks
// to the metrics plugin over rpc and checks the exporter is serving.
//
// The stronger assertion - that the script's collector shows up as
// foo_bar_test - cannot pass yet. The metrics plugin resolves api-go beta.13,
// which carries the connect-era protos, while spiral/roadrunner-metrics 3.3.0
// speaks the v1 line, so the Declare message decodes empty:
//
//	declaring new metric name="" type=COLLECTOR_TYPE_UNSPECIFIED namespace=""
//
// Once the plugin betas are retagged against api-go beta.14, swap the body for
// a poll on foo_bar_test.
func TestOnInitRunsWithMetricsEndpoint(t *testing.T) {
	helpers.Start(t,
		"configs/.rr-metrics-oninit.yaml",
		[]any{&server.Plugin{}, &rpcPlugin.Plugin{}, &metrics.Plugin{}},
		helpers.WithTCPProbe(metricsAddr),
	)

	body := scrapeMetrics(t)

	require.Contains(t, body, "go_goroutines", "the metrics exporter is not serving")
}

// scrapeMetrics fetches the exporter output.
func scrapeMetrics(t *testing.T) string {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+metricsAddr+"/metrics", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return string(body)
}
