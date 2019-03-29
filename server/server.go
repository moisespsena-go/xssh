package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/robfig/cron"
	"github.com/satori/go.uuid"

	"github.com/go-errors/errors"
	"github.com/moisespsena-go/httpu"
	"github.com/moisespsena-go/task"

	"github.com/moisespsena-go/xssh/server/updater"

	"github.com/gliderlabs/ssh"
)

type Server struct {
	KeyFile        string
	Addr           string
	HttpConfig     *httpu.Config
	SocketsDir     string
	NodeSockerPerm os.FileMode
	Updater        updater.Updater

	Cron               *cron.Cron
	RenewTokenSchedule cron.Schedule

	Users         *Users
	LoadBalancers *LoadBalancers
	register      *DefaultReversePortForwardingRegister
	HttpHosts     *HttpHosts

	srv        *ssh.Server
	ln         net.Listener
	running    bool
	httpServer *httpu.Server
}

func (srv *Server) CreateToken() (err error) {
	var exists bool
	if _, err = os.Stat("xssh.token"); err == nil {
		exists = true
	}
	token := uuid.NewV1().String() + "-" + uuid.NewV4().String()
	err = ioutil.WriteFile("xssh.token", []byte(token), 0600)
	if err != nil {
		if exists {
			return fmt.Errorf("renew token file failed: %v", err)
		}
		return fmt.Errorf("create token file failed: %v", err)
	}
	if exists {
		log.Println("token updated")
	} else {
		log.Println("token created")
	}
	return
}

func (srv *Server) Setup(appender task.Appender) (err error) {
	if _, err = os.Stat("xssh.token"); err != nil {
		if os.IsNotExist(err) {
			if err = srv.CreateToken(); err != nil {
				return err
			}
			err = nil
		} else {
			return fmt.Errorf("stat of token failed: %v", err)
		}
	}

	if srv.HttpHosts == nil {
		srv.HttpHosts = &HttpHosts{}
	}

	srv.register = &DefaultReversePortForwardingRegister{
		Nodes: &Nodes{
			Dir:      srv.SocketsDir,
			SockPerm: srv.NodeSockerPerm,
		},
		HttpHosts: srv.HttpHosts,
	}

	if err := os.RemoveAll(srv.SocketsDir); err != nil {
		if !os.IsNotExist(err) {
			return errors.New("remove `" + srv.SocketsDir + "` failed: " + err.Error())
		}
	}

	if srv.ln, err = net.Listen("tcp", srv.Addr); err != nil {
		return
	} else {
		log.Printf("starting ssh server on %v", srv.ln.Addr())
	}

	if srv.HttpConfig != nil {
		srv.httpServer = httpu.NewServer(srv.HttpConfig, srv)
		appender.AddTask(srv.httpServer)
	}

	if srv.RenewTokenSchedule != nil {
		if srv.Cron == nil {
			srv.Cron = cron.New()

			_ = appender.AddTask(task.NewTask(func() (err error) {
				srv.Cron.Start()
				return nil
			}, srv.Cron.Stop))
		}

		srv.Cron.Schedule(srv.RenewTokenSchedule, cron.FuncJob(func() {
			if err := srv.CreateToken(); err != nil {
				log.Println("ERROR", err.Error())
			}
		}))
	}

	srv.setupSshServer()
	return nil
}

func (srv *Server) Run() (err error) {
	srv.running = true
	defer func() {
		srv.running = false
		srv.ln.Close()
	}()
	return srv.srv.Serve(srv.ln)
}

func (srv *Server) Start(done func()) (stop task.Stoper, err error) {
	go func() {
		defer done()
		srv.Run()
	}()
	return srv, nil
}

func (srv *Server) Stop() {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	if srv.httpServer != nil {
		go srv.httpServer.Shutdown(ctx)
	}
	go srv.srv.Shutdown(ctx)
}

func (srv *Server) IsRunning() bool {
	return srv.running
}
