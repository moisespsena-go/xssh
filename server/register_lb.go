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
	connections int
	mu          sync.Mutex
}

func (sl *NodeServiceListener) Release() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.connections--
}

func (sl *NodeServiceListener) Lock() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.connections++
}

func (sl *NodeServiceListener) Dial(ctx context.Context, remoteAddr string) (conn net.Conn, err error) {
	return sl.ServiceListener.Listener.(*ChanListener).Dial(ctx, remoteAddr)
}

type Nodes struct {
	Dir      string
	data     map[string]map[string]*Node
	Ln       net.Listener
	SockPerm os.FileMode
}

func (ns *Nodes) Count(ap, service string) int {
	if ns.data == nil || ns.data[ap] == nil || ns.data[ap][service] == nil {
		return 0
	}
	return len(ns.data[ap][service].EndPoints)
}

func (ns *Nodes) Add(LB *LoadBalancer, ln *ServiceListener) (node *Node, err error) {
	if ns.Count(LB.Ap, LB.Service) >= LB.MaxCount {
		return nil, errors.New("Load balancer endpoints overflowing")
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
	ns.data[LB.Ap][LB.Service].EndPoints[ln.Addr().String()] = &NodeServiceListener{ServiceListener: ln}
	ln.node = n
	return n, nil
}

func (ns *Nodes) Remove(LB *LoadBalancer, ln *NodeServiceListener) {
	ap, service := LB.Ap, LB.Service

	if ns.data == nil || ns.data[ap] == nil || ns.data[ap][service] == nil || ns.data[ap][service].EndPoints[ln.Addr().String()] == nil {
		return
	}

	delete(ns.data[ap][service].EndPoints, ln.Addr().String())
	if len(ns.data[ap][service].EndPoints) == 0 {
		ns.data[ap][service].Close()
		delete(ns.data[ap], service)

		if len(ns.data[ap]) == 0 {
			delete(ns.data, ap)
		}
	}
}
