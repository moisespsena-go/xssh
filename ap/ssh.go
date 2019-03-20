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

func SSHServer(keyFile string) (srvc *Service) {
	srv := &ssh.Server{
		Handler: sshHandler,
		LocalPortForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return true
		},
		ReversePortForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return true
		},
		ConnCallback: func(conn net.Conn) net.Conn {
			var i interface{} = conn
			i.(ssh.CloseListener).CloseCallback(func() {
				log.Println("connection ", conn.RemoteAddr(), "->", conn.LocalAddr(), "closed")
			})
			log.Println("new connection ", conn.RemoteAddr(), "->", conn.LocalAddr())
			return conn
		},
	}
	_ = srv.SetOption(ssh.HostKeyFile(common.GetKeyFile(keyFile)))
	// TODO: Authenticate user
	/*_ = c.srv.SetOption(ssh.PasswordAuth(func(ctx ssh.Context, password string) bool {
		if ctx.User() != currentUser.Name {
			user, err := user.Lookup(ctx.User())
			if err != nil {
				return false
			}
			cmd.SysProcAttr.Credential = syscall.Credential{

			}
		}
	}))*/
	//_ = srv.SetOption(ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
	//	return true // allow all keys, or use ssh.KeysEqual() to compare against known keys
	//}))

	return (&Service{
		Name: "ssh",
		ForeverFunc: func(sl *ServiceListener) {
			if err := srv.Serve(sl.Listener); err != nil {
				if err != io.EOF {
					log.Println("["+sl.ID+"] forever failed:", err)
				}
			}
		},
	}).OnClose(func() {
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		srv.Shutdown(ctx)
	})
}
