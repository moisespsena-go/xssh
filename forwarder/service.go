package forwarder

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/moisespsena-go/task"

	"github.com/moisespsena-go/xssh/common"
)

type IsServiceRunningError struct {
	name string
}

func (e IsServiceRunningError) Error() string {
	return "service " + e.name + " is running"
}

type Service struct {
	Name    string
	Addr    string
	ln      net.Listener
	mu      sync.Mutex
	fw      *Forwarder
	stop    bool
	running bool
	onDone  func()
	con net.Conn

	postStart func()
}

func (s *Service) PostTaskStart(r *task.Runner) {
	panic("implement me")
}

func NewService(name string, addr string) (*Service, error) {
	if addr == "" {
		addr = ":"
	}
	parts := strings.SplitN(addr, ":", 2)
	if parts[0] == "" {
		parts[0] = "127.0.0.1"
	}
	if parts[1] == "" {
		parts[1] = "0"
	}

	if parts[1] == "0" {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return nil, err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, err
		}
		defer l.Close()
		parts[1] = fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
	}

	return &Service{Name: name, Addr: strings.Join(parts, ":")}, nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stop = true
}

func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Service) Setup(appender task.Appender) error {
	return nil
}

func (s *Service) Run() (err error) {
	s.stop = false
	if s.running {
		return &IsServiceRunningError{s.Name}
	}
	if err = s.Listen(); err != nil {
		return
	}
	return s.forever()
}

func (s *Service) Start(done func()) (stop task.Stoper, err error) {
	if s.running {
		return nil, &IsServiceRunningError{s.Name}
	}
	if err = s.Listen(); err != nil {
		return
	}
	s.onDone = done

	go s.forever()

	return s, nil
}

func (s *Service) forever() error {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.running = false
		s.ln.Close()
		s.ln = nil
		if s.con != nil {
			s.con.Close()
			s.con = nil
		}

		if s.onDone != nil {
			s.onDone()
			s.onDone = nil
		}
	}()

	for !s.stop {
		conn, err := s.ln.Accept()
		if err != nil {
			log.Println("["+s.Name+"] accept local connection failed:", err)
			return err
		}
		log.Println("["+s.Name+"] new connection", conn.RemoteAddr())
		go s.forward(conn)
	}
	return nil
}

func (s *Service) Listen() (err error) {
	s.ln, err = net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatalln("["+s.Name+"] listen to "+s.Addr+" failed:", err)
		return
	}
	s.Addr = s.ln.Addr().String()

	log.Println("["+s.Name+"] listening on", s.ln.Addr())
	return
}

func (s *Service) getConn() (conn net.Conn, err error) {
	if s.con != nil {
		return s.con, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.con != nil {
		return s.con, nil
	}
	if s.fw.client == nil {
		return nil, io.EOF
	}

	s.con, err = s.fw.client.Dial("unix", s.Name)
	if err != nil {
		log.Println("["+s.Name+"] Connect to remote proxy server failed:", err)
		return
	}
	return s.con, nil
}

func (s *Service) forward(localConn net.Conn) {
	defer log.Println("["+s.Name+"] new connection", localConn.RemoteAddr(), "closed")
	defer localConn.Close()
	remoteConn, err := s.getConn()
	if err != nil {
		return
	}
	la := localConn.LocalAddr().String()
	go common.NewCopier("["+s.Name+"] remote > "+la, localConn, remoteConn).Copy()
	common.NewCopier("["+s.Name+"] "+la+" > remote", remoteConn, localConn).Copy()
}
