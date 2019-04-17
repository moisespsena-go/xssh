package server

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
)

type NodeServiceListener struct {
	*ServiceListener
	key         string
	connections int
	mu          sync.Mutex
}

func (sl *NodeServiceListener) Dial(ctx context.Context, remoteAddr string) (conn net.Conn, err error) {
	conn, err = sl.ServiceListener.Listener.(*ChanListener).Dial(ctx, remoteAddr)
	if err == nil {
		sl.mu.Lock()
		defer sl.mu.Unlock()
		sl.connections++
		conn2 := &conWithClosers{Conn: conn}
		conn2.OnClose(func() {
			sl.mu.Lock()
			defer sl.mu.Unlock()
			sl.connections--
		})
		conn = conn2
	}
	return
}

type conWithClosers struct {
	net.Conn
	closers []func()
}

func (n *conWithClosers) OnClose(f ...func()) {
	n.closers = append(n.closers, f...)
}

func (n *conWithClosers) Close() error {
	defer func() {
		for _, f := range n.closers {
			f()
		}
		n.closers = nil
	}()
	return n.Conn.Close()
}

type Nodes struct {
	Dir      string
	data     map[string]map[string]*Node
	Ln       net.Listener
	SockPerm os.FileMode
	mu       sync.RWMutex
}

func (ns *Nodes) Count(ap, service string) int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if ns.data == nil || ns.data[ap] == nil || ns.data[ap][service] == nil {
		return 0
	}
	return len(ns.data[ap][service].EndPoints)
}

func (ns *Nodes) Add(LB *LoadBalancer, ln *ServiceListener) (node *Node, err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.Count(LB.Ap, LB.Service) >= LB.MaxCount {
		return nil, errors.New("load balancer endpoints overflowing")
	}
	if ns.data == nil {
		ns.data = map[string]map[string]*Node{}
	}
	if _, ok := ns.data[LB.Ap]; !ok {
		ns.data[LB.Ap] = map[string]*Node{}
	}
	var (
		n  *Node
		ok bool
	)

	if n, ok = ns.data[LB.Ap][LB.Service]; !ok {
		var publicAddr string
		if LB.PublicAddr == nil || *LB.PublicAddr == "" {
			publicAddr = ""
		} else {
			publicAddr = *LB.PublicAddr
		}

		n = &Node{
			ChanListener: NewChanListener(LB.Ap + "/" + LB.Service),
			nodes:        ns,
			Dir:          ns.Dir,
			Ap:           LB.Ap,
			Service:      LB.Service,
			EndPoints:    map[string]*NodeServiceListener{},
		}
		if LB.UnixSocket {
			ul := &UnixListener{
				SocketPath: filepath.Join(ns.Dir, LB.Ap, LB.Service+".sock"),
				SockPerm:   ns.SockPerm,
			}
			ul.Str = n.String() + "@" + ul.SocketPath
			n.Listeners = append(n.Listeners, ul)
		}
		if publicAddr != "" {
			pl := &AddrListener{
				AddrS: publicAddr,
			}
			pl.StrPrefix = n.String() + "@"
			n.Listeners = append(n.Listeners, pl)
		}

		ns.data[LB.Ap][LB.Service] = n
		if err = n.Listen(); err != nil {
			return
		}
		go n.Forever()
	}
	key := ln.Addr().String()
	ns.data[LB.Ap][LB.Service].EndPoints[key] = &NodeServiceListener{ServiceListener: ln, key: key}
	ln.node = n
	return n, nil
}

func (ns *Nodes) Remove(LB *LoadBalancer, ln *NodeServiceListener) {
	ap, service := LB.Ap, LB.Service

	if !func() bool {
		ns.mu.RLock()
		defer ns.mu.RUnlock()

		if ns.data == nil || ns.data[ap] == nil || ns.data[ap][service] == nil || ns.data[ap][service].EndPoints[ln.key] == nil {
			return false
		}
		return true
	}() {
		return
	}

	ns.mu.Lock()
	defer ns.mu.Unlock()

	delete(ns.data[ap][service].EndPoints, ln.key)

	if len(ns.data[ap][service].EndPoints) == 0 {
		ns.data[ap][service].Close()
		delete(ns.data[ap], service)

		if len(ns.data[ap]) == 0 {
			delete(ns.data, ap)
		}
	}
}
