package server

import (
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

func (n Node) CloseEndPont(addr string) {
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

	var (
		first = true
		min   int
		minSL *NodeServiceListener
	)

	for _, sl := range n.EndPoints {
		if first {
			min = sl.connections
			minSL = sl
			first = false
		} else {
			if sl.connections < min {
				minSL = sl
			}
		}
	}

	minSL.Lock()
	defer minSL.Release()

	n.mu.Unlock()

	var (
		addrs = minSL.Addr().String()

		rCon net.Conn
		err  error
	)
	rCon, err = minSL.Dial(nil, conn.RemoteAddr().String())

	if err != nil {
		log.Println(prfx, "dial to", minSL.Name, "failed:", err.Error())
		return
	}

	rprfx := prfx + " " + minSL.Name + "@" + "{" + addrs + "}"

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
