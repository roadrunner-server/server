package server

import (
	"context"

	"github.com/roadrunner-server/pool/payload"
	"github.com/roadrunner-server/pool/pool"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	"github.com/roadrunner-server/pool/worker"
	"go.uber.org/zap"
)

// Pool manager sets of inner worker processes.
type Pool interface {
	// GetConfig returns pool configuration.
	GetConfig() *pool.Config
	// Workers return a worker list associated with the pool.
	Workers() (workers []*worker.Process)
	// RemoveWorker removes worker from the pool.
	RemoveWorker(ctx context.Context) error
	// AddWorker adds worker to the pool.
	AddWorker() error
	// Exec payload
	Exec(ctx context.Context, p *payload.Payload, stopCh chan struct{}) (chan *staticPool.PExec, error)
	// Reset kills all workers inside the watcher and replaces with new
	Reset(ctx context.Context) error
	// Destroy all underlying stacks (but let them complete the task).
	Destroy(ctx context.Context)
}

const (
	// PluginName for the server
	PluginName string = "server"
	// RPCPluginName is the name of the RPC plugin, should be in sync with rpc/config.go
	RPCPluginName string = "rpc"
	// RrRelay env variable key (internal)
	RrRelay string = "RR_RELAY"
	// RrRPC env variable key (internal) if the RPC presents
	RrRPC string = "RR_RPC"
	// RrVersion env variable
	RrVersion string = "RR_VERSION"

	// internal
	delim string = "://"
	unix  string = "unix"
	tcp   string = "tcp"
	pipes string = "pipes"
)

type Configurer interface {
	// UnmarshalKey takes a single key and unmarshal it into a Struct.
	UnmarshalKey(name string, out any) error
	// Has checks if a config section exists.
	Has(name string) bool
	// RRVersion is the roadrunner current version
	RRVersion() string
}

type NamedLogger interface {
	NamedLogger(name string) *zap.Logger
}
