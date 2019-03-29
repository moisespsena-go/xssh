package ap

import (
	"context"
	"io"
	"log"
	"net"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/moisespsena-go/xssh/common"
)

type SSHListener struct {
	Source chan net.Conn
}

func NewSSHListener(source chan net.Conn) *SSHListener {
	return &SSHListener{Source: source}
}

func (l *SSHListener) Accept() (net.Conn, error) {
	if l.Source == nil {
		return nil, io.EOF
	}
	con := <-l.Source
	if con == nil {
		return nil, io.EOF
	}
	return con, nil
}

func (l *SSHListener) Close() error {
	return nil
}

func (l *SSHListener) Addr() (addr net.Addr) {
	return &net.TCPAddr{}
}

func SSHServer(keyFile string) (srvc *Service, closer io.Closer) {
	srv := &ssh.Server{
		Handler: sshHandler,
		SocketForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return true
		},
		ReverseSocketForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return true
		},
		ConnCallback: func(conn net.Conn) net.Conn {
			var i interface{} = conn
			cl := i.(ssh.CloseListener)
			cl.CloseCallback(func() {
				log.Println("connection ", conn.RemoteAddr(), "->", conn.LocalAddr(), "closed")
			})

			log.Println("new connection ", conn.RemoteAddr(), "->", conn.LocalAddr())
			return conn
		},
	}

	_ = srv.SetOption(ssh.HostKeyFile(common.GetKeyFile(keyFile)))

	cc := make(chan net.Conn)
	ln := NewSSHListener(cc)
	go func() {
		if err := srv.Serve(ln); err != nil {
			if err != io.EOF {
				log.Println("[ssh] forever failed:", err)
			}
		}
	}()

	return (&Service{
			Name: "ssh",
			ForeverFunc: func(sl *ServiceListener) {
				for {
					con, err := sl.Accept()
					if err != nil {
						if err != io.EOF {
							log.Println("["+sl.ID+"] forever failed:", err)
						}
					} else {
						cc <- con
					}
				}
			},
		}).OnClose(func() {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			srv.Shutdown(ctx)
		}), IOCloser(func() error {
			close(cc)
			return nil
		})
}

type IOCloser func() error

func (c IOCloser) Close() error {
	return c()
}
