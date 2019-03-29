package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/http2"
	"golang.org/x/net/websocket"
)

var (
	errUnauthorised = errors.New("unauthorised")
)

type Dialer func(ctx context.Context, remoteAddr string) (con net.Conn, err error)

type LB struct {
	*LoadBalancer
	Dial Dialer
}

type HostPaths struct {
	host   string
	paths  map[string]*LB
	sorted []string
	mu     sync.RWMutex
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

func (hp *HostPaths) Set(lb *LoadBalancer, dialer Dialer) {
	var pth = lb.HttpPath
	if pth == "" {
		pth = "/"
	}
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if hp.paths == nil {
		hp.paths = map[string]*LB{}
	}
	hp.sorted = append(hp.sorted, pth)
	hp.paths[pth] = &LB{lb, dialer}
	// reversed
	sort.Slice(hp.sorted, func(i, j int) bool {
		return hp.sorted[i] > hp.sorted[j]
	})
	log.Println("HTTP: <" + hp.host + "> path `" + pth + "` added")
}

func (hp *HostPaths) Remove(pth ...string) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if hp.paths == nil {
		return
	}
	for _, pth := range pth {
		if pth == "" {
			pth = "/"
		}
		if _, ok := hp.paths[pth]; ok {
			delete(hp.paths, pth)
			var sorted []string
			for _, v := range hp.sorted {
				if v == pth {
					log.Println("HTTP: <" + hp.host + "> path `" + pth + "` removed")
				} else {
					sorted = append(sorted, v)
				}
			}
			hp.sorted = sorted
		}
	}
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
	log.Println("HTTP: host `" + host + "` added")
	return hp
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
			log.Println("HTTP: host `" + host + "` removed")
		}
	}
}

func (srv *Server) serveLocal(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if parts := strings.Split(auth, " "); len(parts) == 2 && parts[0] == "Token" && parts[1] != "" {
		b, err := ioutil.ReadFile("xssh.token")
		if err != nil {
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			} else {
				log.Println("read token failed:", err)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		if strings.TrimSpace(string(b)) != parts[1] {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ap, service := r.Header.Get("X-Ap"), r.Header.Get("X-Service")
	if ap == "" {
		http.Error(w, "AP is blank", http.StatusBadRequest)
		return
	}
	if service == "" {
		http.Error(w, "AP is blank", http.StatusBadRequest)
		return
	}

	var clientAddr string

	if parts := strings.Split(service, "/"); len(parts) == 2 {
		service, clientAddr = parts[0], parts[1]
	}

	ln, err := srv.register.GetListener(ap, service, clientAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var cl *ChanListener
	if clientAddr == "" && ln.node != nil {
		cl = ln.node.ChanListener
	} else {
		cl = ln.Listener.(*ChanListener)
	}
	con, _ := cl.Dial(nil, "ws:"+r.RemoteAddr+"->"+r.Host)

	websocket.Handler(func(ws *websocket.Conn) {
		defer con.Close()
		defer ws.Close()
		go func() {
			defer con.Close()
			defer ws.Close()
			io.Copy(ws, con)
		}()
		io.Copy(con, ws)
	}).ServeHTTP(w, r)
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isWebsocketRequest(r) {
		srv.serveLocal(w, r)
		return
	}
	host := strings.SplitN(r.Host, ":", 2)[0]
	var lb *LB
	if pths, ok := srv.HttpHosts.Get(host); ok {
		for _, pth := range pths.sorted {
			if strings.HasPrefix(r.RequestURI, pth) {
				if lb_, ok := pths.Get(pth); ok {
					lb = lb_
					break
				}
			}
		}
	}
	if lb == nil {
		http.NotFound(w, r)
		return
	}

	resp, err := srv.RoundTrip(lb, r)
	if err != nil {
		if err == errUnauthorised {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"User Visible Realm\"")
			http.Error(w, err.Error(), http.StatusUnauthorized)
		} else {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}

	var close = resp.Body.Close

	defer func() {
		fmt.Println("done")
		close()
	}()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	var n int64

	if len(resp.TransferEncoding) == 1 && resp.TransferEncoding[0] == "chunked" {
		n, err = io.Copy(
			ioutil.Discard,
			httputil.NewChunkedReader(
				io.TeeReader(
					resp.Body,
					&flushWriter{w},
				),
			),
		)
	} else {
		n, err = io.Copy(&flushWriter{w}, resp.Body)
	}

	prfx := fmt.Sprintf("[%s{%s}@%s]", lb.Ap, lb.Service, r.RemoteAddr)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		log.Println(prfx, "error:", err.Error())
	} else {
		log.Println(prfx, "transfered", n, "bytes")
	}
}

// RoundTrip is http.RoundTriper implementation.
func (srv *Server) RoundTrip(lb *LB, r *http.Request) (resp *http.Response, err error) {
	var (
		users   HttpUsers
		enabled bool
	)
	if users, enabled, err = srv.LoadBalancers.GetUsers(lb.Ap, lb.Service); err != nil {
		return
	}

	outr := r.WithContext(r.Context())
	if r.ContentLength == 0 {
		outr.Body = nil // Issue 16036: nil Body for http.Transport retries
	}
	outr.Header = cloneHeader(r.Header)

	if enabled {
		if user, password, ok := r.BasicAuth(); !ok || !users.Match(user, password) {
			return nil, errUnauthorised
		}

		outr.Header.Del("Authorization")
	}

	setXForwardedFor(outr.Header, r.RemoteAddr)
	scheme := r.URL.Scheme
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	outr.URL.Host = r.Host
	outr.URL.Scheme = scheme
	if r.Header.Get("X-Forwarded-Host") == "" {
		outr.Header.Set("X-Forwarded-Host", r.Host)
		outr.Header.Set("X-Forwarded-Proto", scheme)
	}
	if r.Header.Get("X-Root-Path") == "" && lb.HttpPath != "" && lb.HttpPath != "/" {
		outr.Header.Set("X-Root-Path", lb.HttpPath)
	}
	outr.RequestURI = ""

	return srv.proxyHTTP(lb, outr)
}

func (s *Server) proxyHTTP(lb *LB, r *http.Request) (resp *http.Response, err error) {
	var t http.RoundTripper
	if r.Proto == "HTTP/2" {
		var con net.Conn
		con, err = lb.Dial(nil, r.RemoteAddr)
		if err != nil {
			return
		}

		defer con.Close()
		t := &http2.Transport{
			AllowHTTP: true,
		}
		var ccon *http2.ClientConn
		if ccon, err = t.NewClientConn(con); err != nil {
			return
		}
		t.ConnPool = &httpConnPool{ccon}
	} else {
		t = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
				return lb.Dial(ctx, r.RemoteAddr)
			},
		}
	}
	httpClient := &http.Client{
		Transport: t,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err = httpClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("io error: %s", err)
	}

	return resp, nil
}

type httpConnPool struct {
	con *http2.ClientConn
}

func (cp *httpConnPool) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	return cp.con, nil
}

func (cp *httpConnPool) MarkDead(*http2.ClientConn) {
}
