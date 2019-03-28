package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/moisespsena-go/xssh/common"
	"github.com/moisespsena-go/xssh/server/updater"
	gossh "golang.org/x/crypto/ssh"
)

func (srv *Server) setupSshServer() {
	var register = srv.register

	srv.srv = &ssh.Server{
		Handler: func(s ssh.Session, request *gossh.Request) {
			var (
				uc   = updater.NewUpdaterClient(s, "AP "+s.User())
				args = s.Command()
			)

			if len(args) == 0 {
				s.Stderr().Write([]byte(`invalid args`))
				return
			}

			switch args[0] {
			case "update":
				if srv.Updater == nil {
					var r common.UpgradePayload
					r.Ok = true
					if err := r.Write(s); err != nil {
						if err != io.EOF {
							log.Println("Write upgrade payload failed: %v", err)
						}
					}
				} else {
					var v common.Version
					if err := v.FRead(s); err != nil {
						uc.Err(err.Error())
						return
					}

					var apUV common.ApUpgradePayload
					apUV.Ap = s.User()
					apUV.ApAddr = s.RemoteAddr().String()
					apUV.Version = v

					if err := srv.Updater.Execute(uc, apUV); err != nil {
						var r common.UpgradePayload
						if err = r.ErrorF(s, err.Error()); err != nil {
							if err != io.EOF {
								log.Println("Write upgrade payload failed: %v", err)
							}
						}
					}
				}
			default:
				s.Stderr().Write([]byte("invalid command"))
			}
		},
		ReverseForwardingRegister: register,
		ReversePortForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return strings.HasPrefix(addr, "unix") && ctx.Value("is:ap").(bool)
		},
		ReverseUnixSocketForwardingListenerCallback: func(ctx ssh.Context, pth string) (listener net.Listener, err error) {
			name := strings.TrimPrefix(pth, "unix:")
			ap := ctx.User()
			var (
				fname    string
				baseName = common.TimeToString(time.Now()) + ".sock"
			)
			if name[0] == '*' {
				serviceName := name[1:]
				var b *LoadBalancer
				if b, err = srv.LoadBalancers.Get(ap, serviceName); err != nil {
					err = fmt.Errorf("LoadBalancers.Get(%q, %q) failed: %v", ap, serviceName)
					return
				} else if b == nil {
					return nil, fmt.Errorf("Load Balance of AP %q and service %q not registered", ap, serviceName)
				}
				fname = filepath.Join(ap, serviceName, baseName)
				ctx.SetValue("load_balancer:"+serviceName, b)
			} else {
				fname = filepath.Join(ap, name+"_"+baseName)
			}

			unixListener := &UnixListener{
				Str:        fname,
				RootDir:    srv.SocketsDir,
				SocketPath: filepath.Join(srv.SocketsDir, fname),
				SockPerm:   srv.NodeSockerPerm,
			}

			if err = unixListener.Listen(); err != nil {
				log.Printf("[AP %s] {%s} listen on %v failed: %v", ctx.User(), name, unixListener.SocketPath, err.Error())
				return
			}

			log.Printf("[AP %s] {%s} listening on %v", ctx.User(), name, unixListener.SocketPath)

			return unixListener, nil
		},
		ReverseUnixSocketForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return ctx.Value("is:ap").(bool)
		},
		LocalUnixSocketForwardingCallback: func(ctx ssh.Context, addr string) bool {
			return !ctx.Value("is:ap").(bool)
		},
		LocalUnixSocketForwardingResolverCallback: func(ctx ssh.Context, addr string) (destAddr string, err error) {
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
	srv.srv.RequestHandler("", ssh.RequestHandlerFunc(func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
		return true, nil
	}))
	srv.srv.RequestHandler("ap-version", ssh.RequestHandlerFunc(func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
		var v common.Version
		log.Println("[AP " + ctx.User() + "] version=" + fmt.Sprint(*v.Unmarshal(req.Payload)))
		return true, nil
	}))
	srv.srv.RequestHandler("cl-version", ssh.RequestHandlerFunc(func(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
		var v common.Version
		log.Println("[CL " + ctx.User() + "] version=" + fmt.Sprint(*v.Unmarshal(req.Payload)))
		return true, nil
	}))
	_ = srv.srv.SetOption(ssh.HostKeyFile(common.GetKeyFile(srv.KeyFile)))
	_ = srv.srv.SetOption(ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
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
}
