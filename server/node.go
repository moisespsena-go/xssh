package server

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/moisespsena-go/xssh/common"
)

type Node struct {
	*ChanListener
	nodes       *Nodes
	Dir         string
	Ap, Service string
	Listeners   []Listener
	EndPoints   map[string]*NodeServiceListener
	mu          sync.Mutex
}

func (n Node) CloseEndPoint(addr string) {
	if sl, ok := n.EndPoints[addr]; ok {
		n.nodes.Remove(&LoadBalancer{Ap: n.Ap, Service: n.Service}, sl)
	}
}

func (n Node) proxy(conn net.Conn) {
	prfx := n.String()
	defer func() {
		conn.Close()
		log.Println(prfx, "closed")
	}()
	log.Println(prfx, "connected")

	n.mu.Lock()
	if len(n.EndPoints) == 0 {
		log.Println(prfx, "no have endpoints")
		return
	}

	n.mu.Unlock()

	sl, rCon, err := n.NextDialSl(nil, conn.RemoteAddr().String())
	if err != nil {
		log.Println(prfx, "dial failed:", err.Error())
		return
	}
	addrs := sl.Addr().String()
	rprfx := prfx + " " + sl.Name + "@" + "{" + addrs + "}"

	defer func() {
		rCon.Close()
		log.Println(rprfx, "closed")
	}()

	log.Println(n.String(), "EP{"+addrs+"}: connected from", conn.RemoteAddr().String())
	common.NewIOSync(
		common.NewCopier(rprfx+" <", conn, rCon),
		common.NewCopier(rprfx+" >", rCon, conn),
	).Sync()
}

func (n Node) String() string {
	return "LB{" + n.Ap + ":" + n.Service + "}"
}

func (n Node) Listen() (err error) {
	if err = n.ChanListener.Listen(); err != nil {
		return
	}
	var addrs = []string{n.ChanListener.ProtoAddr()}
	for _, l := range n.Listeners {
		if err = l.Listen(); err == nil {
			addrs = append(addrs, l.ProtoAddr())
		}
	}
	log.Println(n.String(), "listening on", "{"+strings.Join(addrs, ", ")+"}")
	return
}

func (n Node) NextDial(ctx context.Context, remoteAddr string) (conn net.Conn, err error) {
	_, conn, err = n.NextDialSl(ctx, remoteAddr)
	return
}

func (n Node) NextDialSl(ctx context.Context, remoteAddr string) (sl *NodeServiceListener, conn net.Conn, err error) {
	var (
		first = true
		min   int
	)

	for _, sl2 := range n.EndPoints {
		if first {
			min = sl2.connections
			sl = sl2
			first = false
		} else {
			if sl2.connections < min {
				sl = sl2
			}
		}
	}

	if conn, err = sl.Dial(nil, remoteAddr); err != nil {
		sl = nil
	}
	return
}

func (n Node) forever(ln Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Println(ln.String(), "accept failed:", err.Error())
			}
			return
		}

		go n.proxy(conn)
	}
}

func (n Node) Forever() {
	for _, l := range n.Listeners {
		go n.forever(l)
	}
	n.forever(n)
}

func (n Node) Close() (err error) {
	for _, l := range n.Listeners {
		l.Close()
	}
	return
}
