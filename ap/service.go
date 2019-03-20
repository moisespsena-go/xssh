package ap

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type ServiceListener struct {
	net.Listener
	ID      string
	s       *Service
	onClose []func()
}

func (ln *ServiceListener) OnClose(f ...func()) *ServiceListener {
	ln.onClose = append(ln.onClose, f...)
	return ln
}

func (ln *ServiceListener) Close() {
	ln.s.mu.Lock()
	defer func() {
		ln.s.mu.Unlock()
		for _, f := range ln.onClose {
			f()
		}
		ln.onClose = nil
	}()
	ln.close()
}

func (ln *ServiceListener) close() {
	if ln.Listener != nil {
		ln.Listener.Close()
	}
	if ln.s.listeners != nil {
		delete(ln.s.listeners, ln.ID)
	}
}

type Service struct {
	Name        string
	Addr        string
	ForeverFunc func(sl *ServiceListener)
	mu          sync.Mutex
	listeners   map[string]*ServiceListener
	lid         int
	onClose     []func()
}

func (s *Service) OnClose(f ...func()) *Service {
	s.onClose = append(s.onClose, f...)
	return s
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		for _, f := range s.onClose {
			f()
		}
		s.onClose = nil
	}()
	if s.listeners != nil {
		for _, ln := range s.listeners {
			ln.close()
		}
	}
	return nil
}

func (s *Service) proxy(sl *ServiceListener, remoteConn net.Conn) {
	prfx := remoteConn.RemoteAddr().String()
	log.Println(prfx, "connected")
	defer func() {
		log.Println(prfx, "closed")
	}()

	if conn, err := net.Dial("tcp", s.Addr); err != nil {
		log.Println(prfx, "net.Dial to", s.Addr, "failed:", err)
		return
	} else {
		go io.Copy(conn, remoteConn)
		io.Copy(remoteConn, conn)
	}
}

func (s *Service) forever(sl *ServiceListener) {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.listeners != nil {
			if _, ok := s.listeners[sl.ID]; ok {
				delete(s.listeners, sl.ID)
			}
		}
	}()
	if s.ForeverFunc != nil {
		s.ForeverFunc(sl)
	} else {
		for sl.Listener != nil {
			conn, err := sl.Accept()
			if err != nil {
				if err != io.EOF {
					log.Println("["+sl.ID+"] accept failed:", err)
				}
				return
			}
			go s.proxy(sl, conn)
		}
	}
}

func (s *Service) Register(prefix string, ln net.Listener) *ServiceListener {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listeners == nil {
		s.listeners = map[string]*ServiceListener{}
	}
	s.lid++
	sl := &ServiceListener{Listener: ln, ID: prefix + "S" + fmt.Sprintf("%02d{%s}", s.lid, s.Name), s: s}
	log.Println("[" + s.Name + "] register listener #" + sl.ID)
	s.listeners[sl.ID] = sl
	go s.forever(sl)
	return sl
}
