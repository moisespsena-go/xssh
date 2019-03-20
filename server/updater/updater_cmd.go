package updater

import (
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/moisespsena-go/xssh/common"
)

type CommandUpdater struct {
	Name string
	Args []string
	Env  []string
}

func NewCommandUpdater(name string, args ...string) *CommandUpdater {
	return &CommandUpdater{Name: name, Args: args}
}

func (c *CommandUpdater) Execute(uc *UpdaterClient, payload common.ApUpgradePayload) (err error) {
	cmd := c.Cmd()
	var (
		w io.WriteCloser
		r io.ReadCloser
	)

	if w, err = cmd.StdinPipe(); err != nil {
		return
	}

	defer w.Close()

	if r, err = cmd.StdoutPipe(); err != nil {
		return
	}

	defer r.Close()
	if err = cmd.Start(); err != nil {
		return err
	}

	go uc.Sync(payload, w, r)
	if err := cmd.Wait(); err != nil {
		uc.Log("cmd.Wait failed: %v", err.Error())
	}
	return
}

func (c *CommandUpdater) Cmd() (cmd *exec.Cmd) {
	cmd = exec.Command(c.Name, c.Args...)
	cmd.Env = append(common.Environ(), c.Env...)
	exe, _ := os.Executable()
	cmd.Env = append(cmd.Env,
		"XSSH_BIN="+exe,
		"XSSH_PID="+strconv.Itoa(os.Getpid()),
	)
	cmd.Stderr = os.Stderr
	return
}
