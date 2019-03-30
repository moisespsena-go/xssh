package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/http2"
)

var (
	errUnauthorised = errors.New("unauthorised")
)

type Dialer func(ctx context.Context, remoteAddr string) (con net.Conn, err error)

func cleanPth(pth string) string {
	if pth == "" {
		pth = "/"
	}
	if !strings.HasSuffix(pth, "/") {
		pth += "/"
	}
	if pth[0] != '/' {
		pth = "/" + pth
	}
	return pth
}

type LB struct {
	*LoadBalancer
	Node *Node
}

type HostPaths struct {
	host   string
	paths  map[string]*LB
	sorted []string
	mu     sync.RWMutex
}

func (hp *HostPaths) sort() {
	var sorted []string
	for pth := range hp.paths {
		sorted = append(sorted, pth)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] > sorted[j]
	})
	hp.sorted = sorted
}

func (hp *HostPaths) Get(pth string) (lb *LB, ok bool) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	if len(hp.sorted) == 0 {
		return
	}
	lb, ok = hp.paths[pth]
	return
}

func (hp *HostPaths) Set(lb *LoadBalancer, n *Node) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if hp.paths == nil {
		hp.paths = map[string]*LB{}
	}
	var pth = lb.HttpPath
	if _, ok := hp.paths[pth]; !ok {
		hp.paths[pth] = &LB{lb, n}
		log.Println("HTTP:", "`"+n.Name()+"`", "mounted to `"+hp.host+pth+"`")
		hp.sort()
	}
}

func (hp *HostPaths) Remove(pth ...string) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if hp.paths == nil {
		return
	}
	for _, pth := range pth {
		if lb, ok := hp.paths[pth]; ok {
			delete(hp.paths, pth)
			log.Println("HTTP:", "`"+lb.Node.Name()+"`{"+hp.host+pth+"}", "umounted")
		}
	}
	hp.sort()
}

type HttpHosts struct {
	hosts map[string]*HostPaths
	mu    sync.RWMutex
}

func (h *HttpHosts) Get(host string) (pths *HostPaths, ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.hosts == nil {
		return
	}
	pths, ok = h.hosts[host]
	return
}

func (h *HttpHosts) Register(host string) *HostPaths {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hosts == nil {
		h.hosts = map[string]*HostPaths{}
	}
	hp := &HostPaths{host: host}
	h.hosts[host] = hp
	return hp
}

func (h *HttpHosts) GetOrRegister(host string) (pths *HostPaths) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.hosts == nil {
		h.hosts = map[string]*HostPaths{}
	}
	var ok bool
	if pths, ok = h.hosts[host]; !ok {
		pths = &HostPaths{host: host}
		h.hosts[host] = pths
	}
	return
}

func (h *HttpHosts) Remove(host string, pth ...string) {
	pths, ok := h.Get(host)
	if !ok {
		return
	}
	pths.Remove(pth...)
	if len(pths.paths) == 0 {
		h.mu.Lock()
		defer h.mu.Unlock()

		if len(pths.paths) == 0 {
			delete(h.hosts, host)
		}
	}
}

type httpConnPool struct {
	con *http2.ClientConn
}

func (cp *httpConnPool) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	return cp.con, nil
}

func (cp *httpConnPool) MarkDead(*http2.ClientConn) {
}
