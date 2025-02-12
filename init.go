package server

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/process"
	"go.uber.org/zap"
)

type command struct {
	log *zap.Logger
	cfg *InitConfig
}

func newCommand(log *zap.Logger, cfg *InitConfig) *command {
	return &command{
		log: log,
		cfg: cfg,
	}
}

func (b *command) start() error {
	const op = errors.Op("server_on_init")
	stopCh := make(chan error, 1)

	cmd := b.createProcess(b.cfg.Env, b.cfg.Command)

	if b.cfg.User != "" {
		err := process.ExecuteFromUser(cmd, b.cfg.User)
		if err != nil {
			return errors.E(op, err)
		}
	}

	timer := time.NewTimer(b.cfg.ExecTimeout)

	err := cmd.Start()
	if err != nil {
		return errors.E(op, err)
	}

	go func() {
		errW := cmd.Wait()
		if errW != nil {
			b.log.Error("process wait", zap.Error(errW))
			stopCh <- errW
			return
		}

		stopCh <- nil
	}()

	select {
	case <-timer.C:
		err = cmd.Process.Kill()
		if err != nil {
			b.log.Error("process killed", zap.Error(err))
			return err
		}

		if b.cfg.ExitOnError {
			return errors.Str("startup process has been killed by timeout")
		}

		return nil

	case err := <-stopCh:
		timer.Stop()
		return err
	}
}

func (b *command) Write(data []byte) (int, error) {
	b.log.Info(string(data))
	return len(data), nil
}

// create command for the process
func (b *command) createProcess(env map[string]string, cmd []string) *exec.Cmd {
	// cmdArgs contain command arguments if the command in the form of: php <command> or ls <command> -i -b
	var cmdArgs []string
	var execCmd *exec.Cmd

	// here we may have 2 cases: command declared as a space separated string or as a slice
	switch len(cmd) {
	// command defined as a space separated string
	case 1:
		// we know that the len is 1, so we can safely use the first element
		cmdArgs = append(cmdArgs, strings.Split(cmd[0], " ")...)
	default:
		// we have a slice with a 2 or more elements
		// first element is the command, the rest are arguments
		cmdArgs = cmd
	}

	if len(cmdArgs) == 1 {
		execCmd = exec.Command(cmd[0])
	} else {
		execCmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	}

	// set env variables from the config
	if len(env) > 0 {
		for k, v := range env {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", strings.ToUpper(k), os.Expand(v, os.Getenv)))
		}
	}

	// append system envs
	execCmd.Env = append(execCmd.Env, os.Environ()...)
	// redirect stderr and stdout into the Write function of the process.go
	execCmd.Stderr = b
	execCmd.Stdout = b

	return execCmd
}
