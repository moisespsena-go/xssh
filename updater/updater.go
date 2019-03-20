package updater

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/jpillora/overseer/fetcher"

	"github.com/moisespsena-go/xssh/common"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Fetcher struct {
	KeyFile    string
	ServerAddr string
	User       string
	signer     ssh.Signer
	URLFetcher func(url string) (f fetcher.Interface, err error)
}

func (f *Fetcher) Init() error {
	buf, err := ioutil.ReadFile(common.GetKeyFile(f.KeyFile))
	if err != nil {
		return fmt.Errorf("load Key failed: %v", err)
	}
	f.signer, err = ssh.ParsePrivateKey(buf)
	if err != nil {
		return fmt.Errorf("parse Key failed: %v", err)
	}
	if f.URLFetcher == nil {
		f.URLFetcher = func(url string) (f fetcher.Interface, err error) {
			f = &fetcher.HTTP{
				URL: url,
			}
			if err = f.Init(); err != nil {
				return nil, err
			}
			return
		}
	}
	return nil
}

func (f *Fetcher) Fetch() (r io.Reader, err error) {
	var (
		stdout, stderr bytes.Buffer
		exe            string
	)
	if exe, err = os.Executable(); err != nil {
		return nil, fmt.Errorf("get executable failed: %v", err)
	}

	cmd := exec.Command(exe, "version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = common.Environ()

	if err = cmd.Run(); err != nil {
		os.Stderr.Write(stderr.Bytes())
		return nil, fmt.Errorf("get current stdout failed: %v", err)
	}

	version := stdout.String()
	stdout.Reset()
	stdout.WriteString(version)
	var v common.Version
	if err = yaml.NewDecoder(&stdout).Decode(&v); err != nil {
		return nil, fmt.Errorf("parse stdout failed: %v", err)
	}

	if v.Digest, err = common.Digest(exe); err != nil {
		return
	}

	stdout.Reset()
	if err = yaml.NewEncoder(&stdout).Encode(&v); err != nil {
		return nil, fmt.Errorf("marshall version failed: %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: f.User,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(f.signer)},
	}

	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", f.ServerAddr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to server failed: %v", err)
	}

	var close = []func() error{client.Close}
	defer func() {
		if err != nil {
			for _, f := range close {
				f()
			}
		}
	}()

	s, err := client.NewSession()
	if err != nil {
		err = fmt.Errorf("create session failed: %v", err)
		return
	}

	close = append(close, s.Close)
	s.Stdin = &stdout
	var out io.Reader
	if out, err = s.StdoutPipe(); err != nil {
		err = fmt.Errorf("pipe session stdout failed: %v", err)
		return
	}
	if err = s.Start("update"); err != nil {
		err = fmt.Errorf("start `upgrade` command failed: %v", err)
		return
	}

	var p common.UpgradePayload
	if r, err = p.Read(out); err != nil {
		err = fmt.Errorf("parse payload failed: %v", err)
		return
	}

	if !p.Ok {
		return nil, p.Err
	}

	if p.Changed {
		if p.Stream {
			return &readCloser{r, close}, nil
		} else if p.URL != "" {
			if f2, err := f.URLFetcher(p.URL); err != nil {
				err = fmt.Errorf("make URL fetcher failed: %v", err)
				return nil, err
			} else {
				return f2.Fetch()
			}
		}
	}

	return
}

type readCloser struct {
	io.Reader
	closers []func() error
}

func (r readCloser) Close() (err error) {
	defer func() {
		for _, c := range r.closers {
			c()
		}
	}()
	if c, ok := r.Reader.(io.ReadCloser); ok {
		err = c.Close()
	}
	return
}
