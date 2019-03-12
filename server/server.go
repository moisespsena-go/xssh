package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/moisespsena-go/xssh/common"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type Server struct {
	KeyFile    string
	Addr       string
	SocketsDir string
	NodeSockerPerm os.FileMode

	Users         *Users
	LoadBalancers *LoadBalancers
	register      *DefaultReversePortForwardingRegister
}

func (srv *Server) Serve() {
	srv.register = &DefaultReversePortForwardingRegister{
		Nodes:&Nodes{
			Dir:filepath.Join(srv.SocketsDir, "lb_nodes"),
			SockPerm:srv.NodeSockerPerm,
		},
	}

	var (
		ssrv     *ssh.Server
		register = srv.register
	)

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}

	log.Printf("starting ssh server on %v", ln.Addr())

	ssrv = &ssh.Server{
		ReversePortForwardingRegister: register,
		ReversePortForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return strings.HasPrefix(addr, "unix") && ctx.Value("is:ap").(bool)
		},
		ReversePortForwardingListenerCallback: func(ctx ssh.Context, addr string) (listener net.Listener, err error) {
			name := strings.TrimPrefix(addr, "unix:")
			ap := ctx.User()
			if name[0] == '*' {
				serviceName := name[1:]
				var b *LoadBalancer
				if b, err = srv.LoadBalancers.Get(ap, serviceName); err != nil {
					err = fmt.Errorf("LoadBalancers.Get(%q, %q) failed: %v", ap, serviceName)
					return
				} else if b == nil {
					return nil, fmt.Errorf("Load Balance of AP %q and service %q not registered", ap, serviceName)
				}
				ctx.SetValue("load_balancer:"+serviceName, b)
				listener, err = net.Listen("tcp", "localhost:0")
			} else {
				listener, err = net.Listen("tcp", "localhost:0")
			}
			if err == nil {
				log.Printf("[AP %s] {%s} listening on %v", ctx.User(), name, listener.Addr())
			}
			return
		},
		LocalPortForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return !ctx.Value("is:ap").(bool)
		},
		LocalPortForwardingResolverCallback: func(ctx ssh.Context, addr string) (destAddr string, err error) {
			apName := ctx.Value("ap:name").(string)
			serviceName := strings.TrimPrefix(addr, "unix:")
			var ln *ServiceListener
			if ln, err = register.GetListener(apName, serviceName); err != nil {
				return
			}
			if ln.node == nil {
				ctx.Value(ssh.ContextKeyCloseListener).(ssh.CloseListener).CloseCallback(ln.Release)
				log.Println("[CL " + ctx.User() + "] -> {" + serviceName + "}")
				return ln.Addr().String(), nil
			}

			log.Println("[CL " + ctx.User() + "] -> " + ln.node.String())
			return "unix:" + ln.node.SocketPath, nil
		},
		ConnCallback: func(conn net.Conn) net.Conn {
			var i interface{} = conn
			i.(ssh.CloseListener).CloseCallback(func() {
				log.Println("connection", conn.RemoteAddr(), "closed")
			})
			log.Println("new connection", conn.RemoteAddr())
			return conn
		},
	}
	ssrv.RequestHandler("", ssh.RequestHandlerFunc(func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
		return true, nil
	}))
	_ = ssrv.SetOption(ssh.HostKeyFile(common.GetKeyFile(srv.KeyFile)))
	_ = ssrv.SetOption(ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
		user := ctx.User()
		parts := strings.Split(user, ":")
		var (
			proxy  bool
			apName string
		)

		if len(parts) >= 2 {
			user = parts[0]
			if parts[1] == "" {
				log.Printf("Ap name of user %q is blank\n", user)
				return false
			}
			apName = parts[1]
			ctx.SetValue("ap:name", apName)
			if len(parts) == 3 {
				if parts[2] == "" {
					parts[2] = parts[0]
				}

				ctx.SetValue("proxy:user", parts[2])
				proxy = true
			}
		}
		if user == "" {
			log.Printf("User is blank\n")
			return false
		}
		err, ok, isAp := srv.Users.CheckUser(user, string(gossh.MarshalAuthorizedKey(key)))
		if err != nil {
			log.Println("ERROR:", err)
			return false
		}
		if ok {
			ctx.SetValue("is:ap", isAp)
			ctx.SetValue("is:proxy", proxy)
		}
		return ok
	}))
	log.Fatal(ssrv.Serve(ln))
}
