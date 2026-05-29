package server

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/pool/v2/process"
)

type command struct {
	log *slog.Logger
	cfg *InitConfig
}

func newCommand(log *slog.Logger, cfg *InitConfig) *command {
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

	err := cmd.Start()
	if err != nil {
		return errors.E(op, err)
	}

	// Start the timer only after the process has launched successfully.
	timer := time.NewTimer(b.cfg.ExecTimeout)
	defer timer.Stop()

	go func() {
		errW := cmd.Wait()
		if errW != nil {
			b.log.Error("process wait", "error", errW)
			stopCh <- errW
			return
		}

		stopCh <- nil
	}()

	select {
	case <-timer.C:
		err = cmd.Process.Kill()
		if err != nil {
			b.log.Error("process killed", "error", err)
			return err
		}

		if b.cfg.ExitOnError {
			return errors.Str("startup process has been killed by timeout")
		}

		return nil

	case err := <-stopCh:
		return err
	}
}

func (b *command) Write(data []byte) (int, error) {
	b.log.Info(string(data))
	return len(data), nil
}

// create command for the process
func (b *command) createProcess(env map[string]string, cmd []string) *exec.Cmd {
	cmdArgs := prepareCmd(cmd)

	var execCmd *exec.Cmd
	if len(cmdArgs) == 1 {
		execCmd = exec.CommandContext(context.Background(), cmdArgs[0])
	} else {
		execCmd = exec.CommandContext(context.Background(), cmdArgs[0], cmdArgs[1:]...)
	}

	// OS env first, then config env so that user config takes precedence.
	execCmd.Env = append(os.Environ(), execCmd.Env...)

	// set env variables from the config
	for k, v := range env {
		execCmd.Env = append(execCmd.Env, strings.ToUpper(k)+"="+os.ExpandEnv(v))
	}

	// redirect stderr and stdout into the Write function of the process.go
	execCmd.Stderr = b
	execCmd.Stdout = b

	return execCmd
}
