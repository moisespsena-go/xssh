package server

import (
	"log"
	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/go-errors/errors"
	"github.com/moisespsena-go/xssh/common"
	gossh "golang.org/x/crypto/ssh"
)

func proxy(register *DefaultReversePortForwardingRegister, ctx ssh.Context, conn *gossh.ServerConn) (proxy ssh.Proxy, ok bool) {
	if isProxy := ctx.Value("is:proxy").(bool); !isProxy {
		return nil, false
	}

	var apName = ctx.Value("ap:name").(string)

	return func(ctx ssh.Context, conn *gossh.ServerConn, chans <-chan gossh.NewChannel, reqs <-chan *gossh.Request) {
		prfx := "[" + apName + "@ssh]"
		lp := func(args ...interface{}) {
			log.Println(append([]interface{}{prfx}, args...)...)
		}

		var getClientMu sync.Mutex
		var client *gossh.Client
		var getClient = func() (err error) {
			getClientMu.Lock()
			defer getClientMu.Unlock()
			if client != nil {
				return nil
			}
			defer func() {
				if err != nil {
					err = errors.New("get `ssh` service connection for AP failed: " + err.Error())
				}
			}()
			var proxyAddr string
			if ln, err := register.GetListener(apName, common.SrvcSSH); err == nil {
				proxyAddr = ln.Addr().String()
			} else {
				lp("get listen failed:", err)
				return err
			}

			sshConfig := &gossh.ClientConfig{
				User: ctx.Value("proxy:user").(string),
			}
			sshConfig.HostKeyCallback = gossh.InsecureIgnoreHostKey()

			client, err = gossh.Dial("tcp", proxyAddr, sshConfig)
			if err != nil {
				lp("dial failed:", err)
				return err
			}

			return nil
		}

		defer func() {
			getClientMu.Lock()
			defer getClientMu.Unlock()
			if client != nil {
				client.Close()
			}
		}()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for req := range reqs {
				if err := getClient(); err != nil {
					req.Reply(false, []byte(err.Error()))
					return
				}
				ok, status, err := client.SendRequest(req.Type, req.WantReply, req.Payload)
				if err != nil {
					lp("[req="+req.Type+"] send request failed:", err.Error())
					continue
				}
				if err = req.Reply(ok, status); err != nil {
					lp("[req="+req.Type+"] reply failed:", err.Error())
				}
			}
		}()

		go func() {
			defer wg.Done()

			for newChan := range chans {
				if err := getClient(); err != nil {
					newChan.Reject(gossh.ConnectionFailed, err.Error())
					return
				}
				rch, rreqs, err := client.OpenChannel(newChan.ChannelType(), newChan.ExtraData())
				if err != nil {
					if ocerr, ok := err.(*gossh.OpenChannelError); ok {
						newChan.Reject(ocerr.Reason, ocerr.Message)
						continue
					} else {
						newChan.Reject(gossh.ConnectionFailed, "server error: "+err.Error())
						return
					}
				}

				ch, reqs, err := newChan.Accept()

				go func() {
					defer rch.Close()
					for req := range rreqs {
						ok, err := ch.SendRequest(req.Type, req.WantReply, req.Payload)
						if err != nil {
							lp("[C < AP: ch="+newChan.ChannelType()+", req="+req.Type+"] send request failed:", err.Error())
						}
						if req.WantReply {
							if err != nil {
								err = req.Reply(false, []byte("send to client failed: "+err.Error()))
							} else {
								err = req.Reply(ok, nil)
							}
							if err != nil {
								lp("[C < AP: ch="+newChan.ChannelType()+", req="+req.Type+"] reply failed:", err.Error())
							}
						}
					}
				}()

				if err != nil {
					conn.Close()
					return
				}

				for req := range reqs {
					ok, err := rch.SendRequest(req.Type, req.WantReply, req.Payload)
					if err != nil {
						lp("[C > AP: ch="+newChan.ChannelType()+", req="+req.Type+"] send request failed:", err.Error())
					}
					if req.WantReply {
						if err != nil {
							err = req.Reply(false, []byte("send to client failed: "+err.Error()))
						} else {
							err = req.Reply(ok, nil)
						}
						if err != nil {
							lp("[C > AP: ch="+newChan.ChannelType()+", req="+req.Type+"] reply failed:", err.Error())
						}
					}
				}
			}
		}()
		wg.Wait()
	}, true
}
