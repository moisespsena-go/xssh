package server

import "sync"

type ClientListeners struct {
	count int
	byName map[string]*ServiceListener
	byAddr map[string]*ServiceListener
	mu sync.Mutex
}

func (cl *ClientListeners) Add(ln ...*ServiceListener) {
	if cl.byAddr == nil {
		cl.byAddr = map[string]*ServiceListener{}
	}
	if cl.byName == nil {
		cl.byName = map[string]*ServiceListener{}
	}

	for _, ln := range ln {
		cl.byAddr[ln.Addr().String()] = ln
		cl.byName[ln.Name] = ln
	}
}

func (cl *ClientListeners) Remove(addr string) (name string, ok bool) {
	if ln, ok := cl.byAddr[addr]; ok {
		delete(cl.byAddr, addr)
		delete(cl.byName, ln.Name)
		return ln.Name, ok
	}
	return
}

func (cl *ClientListeners) Close() {
	for _, ln := range cl.byAddr {
		ln.Close()
	}
	cl.byAddr = nil
	cl.byName = nil
}
