package server

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/ipc/pipe"
	"github.com/roadrunner-server/pool/ipc/socket"
	"github.com/roadrunner-server/pool/pool"
	"github.com/roadrunner-server/pool/process"
	"github.com/roadrunner-server/tcplisten"
	"go.uber.org/zap"
)

type internalCommand func() *exec.Cmd

// should be the same as pool.Command
type internalCmdWithArgs func(command []string) *exec.Cmd

// cmdFactory provides worker command factory associated with given context
func (p *Plugin) cmdFactory(env map[string]string) internalCommand {
	return func() *exec.Cmd {
		var cmd *exec.Cmd

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if len(p.preparedCmd) == 1 {
			cmd = exec.CommandContext(ctx, p.preparedCmd[0])
		} else {
			cmd = exec.CommandContext(ctx, p.preparedCmd[0], p.preparedCmd[1:]...)
		}

		// copy prepared envs
		cmd.Env = make([]string, len(p.preparedEnvs))
		copy(cmd.Env, p.preparedEnvs)

		// append external envs
		if len(env) > 0 {
			for k, v := range env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
			}
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

// customCmd used as and enhancement for the CmdFactory to use with a custom command string (used by default)
func (p *Plugin) customCmd(env map[string]string) internalCmdWithArgs {
	return func(command []string) *exec.Cmd {
		// if no command provided, use the server's one
		if len(command) == 0 {
			command = p.cfg.Command
		}

		var cmd *exec.Cmd

		preparedCmd := make([]string, 0, 5)
		// here we may have 2 cases: command declared as a space-separated string or as a slice
		switch len(command) {
		// command defined as a space-separated string
		case 1:
			// we know that the len is 1, so we can safely use the first element
			preparedCmd = append(preparedCmd, strings.Split(command[0], " ")...)
		default:
			// we have a slice with 2 or more elements
			// first element is the command, the rest are arguments
			preparedCmd = command
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if len(preparedCmd) == 1 {
			cmd = exec.CommandContext(ctx, preparedCmd[0])
		} else {
			cmd = exec.CommandContext(ctx, preparedCmd[0], preparedCmd[1:]...)
		}

		// copy prepared envs
		cmd.Env = make([]string, len(p.preparedEnvs))
		copy(cmd.Env, p.preparedEnvs)

		// append external envs
		if len(env) > 0 {
			for k, v := range env {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
			}
		}

		process.IsolateProcess(cmd)
		// if a user is not empty, and the OS is linux or macOS,
		// execute php worker from that particular user
		if p.cfg.User != "" {
			err := process.ExecuteFromUser(cmd, p.cfg.User)
			if err != nil {
				p.log.Panic("can't execute command from the user", zap.String("user", p.cfg.User), zap.Error(err))
				return nil
			}
		}

		return cmd
	}
}

// creates relay and worker factory.
func initFactory(log *zap.Logger, relay string) (pool.Factory, error) {
	const op = errors.Op("server_plugin_init_factory")
	if relay == "" || relay == pipes {
		return pipe.NewPipeFactory(log), nil
	}

	dsn := strings.Split(relay, delim)
	if len(dsn) != 2 {
		return nil, errors.E(op, errors.Network, errors.Str("invalid DSN (tcp://:6001, unix://file.sock)"))
	}

	lsn, err := tcplisten.CreateListener(relay)
	if err != nil {
		return nil, errors.E(op, errors.Network, err)
	}

	switch dsn[0] {
	// sockets group
	case unix:
		return socket.NewSocketServer(lsn, log), nil
	case tcp:
		return socket.NewSocketServer(lsn, log), nil
	default:
		return nil, errors.E(op, errors.Network, errors.Str("invalid DSN (tcp://:6001, unix://file.sock)"))
	}
}
