package server

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strings"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/v2/ipc/pipe"
	"github.com/roadrunner-server/pool/v2/ipc/socket"
	"github.com/roadrunner-server/pool/v2/pool"
	"github.com/roadrunner-server/pool/v2/process"
	"github.com/roadrunner-server/tcplisten"
)

type internalCommand func() *exec.Cmd

// should be the same as pool.Command
type internalCmdWithArgs func(command []string) *exec.Cmd

// prepareCmd normalises a command slice: a single-element slice whose value is
// a space-separated string is split into individual arguments; a multi-element
// slice is used as-is.
func prepareCmd(command []string) []string {
	if len(command) == 1 {
		return strings.Split(command[0], " ")
	}
	return command
}

// cmdFactory provides worker command factory associated with given context
func (p *Plugin) cmdFactory(env map[string]string) internalCommand {
	return func() *exec.Cmd {
		var cmd *exec.Cmd

		if len(p.preparedCmd) == 1 {
			cmd = exec.CommandContext(context.Background(), p.preparedCmd[0])
		} else {
			cmd = exec.CommandContext(context.Background(), p.preparedCmd[0], p.preparedCmd[1:]...)
		}

		// copy prepared envs
		cmd.Env = slices.Clone(p.preparedEnvs)

		// append external envs
		for k, v := range env {
			cmd.Env = append(cmd.Env, strings.ToUpper(k)+"="+v)
		}

		process.IsolateProcess(cmd)
		// if the user is not empty, and the OS is linux or macOS,
		// execute php worker from that particular user
		if p.cfg.User != "" {
			err := process.ExecuteFromUser(cmd, p.cfg.User)
			if err != nil {
				panic(fmt.Errorf("RoadRunner can't execute process from the specified user: %s, error: %w", p.cfg.User, err))
			}
		}

		return cmd
	}
}

// customCmd used as an enhancement for the CmdFactory to use with a custom command string (used by default)
func (p *Plugin) customCmd(env map[string]string) internalCmdWithArgs {
	return func(command []string) *exec.Cmd {
		// if no command provided, use the server's one
		if len(command) == 0 {
			command = p.cfg.Command
		}

		preparedCmd := prepareCmd(command)

		var cmd *exec.Cmd
		if len(preparedCmd) == 1 {
			cmd = exec.CommandContext(context.Background(), preparedCmd[0])
		} else {
			cmd = exec.CommandContext(context.Background(), preparedCmd[0], preparedCmd[1:]...)
		}

		// copy prepared envs
		cmd.Env = slices.Clone(p.preparedEnvs)

		// append external envs
		for k, v := range env {
			cmd.Env = append(cmd.Env, strings.ToUpper(k)+"="+v)
		}

		process.IsolateProcess(cmd)
		// if a user is not empty, and the OS is linux or macOS,
		// execute php worker from that particular user
		if p.cfg.User != "" {
			err := process.ExecuteFromUser(cmd, p.cfg.User)
			if err != nil {
				panic(fmt.Errorf("RoadRunner can't execute process from the specified user: %s, error: %w", p.cfg.User, err))
			}
		}

		return cmd
	}
}

// creates relay and worker factory.
func initFactory(log *slog.Logger, relay string) (pool.Factory, error) {
	const op = errors.Op("server_plugin_init_factory")
	if relay == "" || relay == pipes {
		return pipe.NewPipeFactory(log), nil
	}

	network, _, ok := strings.Cut(relay, delim)
	if !ok {
		return nil, errors.E(op, errors.Network, errors.Str("invalid DSN (tcp://:6001, unix://file.sock)"))
	}

	lsn, err := tcplisten.CreateListener(relay)
	if err != nil {
		return nil, errors.E(op, errors.Network, err)
	}

	switch network {
	case unix, tcp:
		return socket.NewSocketServer(lsn, log), nil
	default:
		return nil, errors.E(op, errors.Network, errors.Str("invalid DSN (tcp://:6001, unix://file.sock)"))
	}
}
