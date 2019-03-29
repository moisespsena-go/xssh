package ap

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/moisespsena-go/xssh/common"

	"github.com/gliderlabs/ssh"
	"github.com/kr/pty"
	gossh "golang.org/x/crypto/ssh"
)

func sshHandler(s ssh.Session) {
	req := s.Request()
	cmds := s.Command()
	var cmd *exec.Cmd
	var iw io.WriteCloser
	if len(cmds) > 0 {
		var payload = struct{ Value string }{}
		gossh.Unmarshal(req.Payload, &payload)
		cmd = exec.Command("bash", "-c", payload.Value)
		iw, _ = cmd.StdinPipe()
	} else {
		cmd = exec.Command("bash")
	}
	cmd.Env = os.Environ()
	cmd.Dir = common.CurrentUser.HomeDir

	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
		f, err := pty.Start(cmd)
		if err != nil {
			fmt.Fprintln(s.Stderr(), "Start failure:", err)
			s.Exit(1)
		}
		go func() {
			for win := range winCh {
				setWinsize(f, win.Width, win.Height)
			}
		}()
		go func() {
			io.Copy(f, s) // stdin
		}()
		go io.Copy(s, f) // stdout
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		opr, opw, _ := os.Pipe()
		cmd.Stdout = opw

		epr, epw, _ := os.Pipe()
		cmd.Stderr = epw

		go io.Copy(s.Stderr(), epr)
		go io.Copy(s, opr)

		if err := cmd.Start(); err != nil {
			fmt.Fprintln(s.Stderr(), "Start failure:", err)
			s.Exit(1)
			return
		}

		if iw != nil {
			go func() {
				defer iw.Close()
				io.Copy(iw, s)
			}()
		}
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				s.Exit(status.ExitStatus())
				return
			}
		}
		s.Exit(1)
		return
	} else {
		s.Exit(0)
	}
}

type reader struct {
	r io.ReadCloser
}

func (r reader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	if n > 0 {
		fmt.Println(strconv.Quote(string(p[0:n])))
	}
	return
}

func (r reader) Close() error {
	return r.r.Close()
}
