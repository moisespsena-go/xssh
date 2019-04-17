package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/moisespsena-go/httpu"
	"golang.org/x/net/http2"
	"golang.org/x/net/websocket"
)

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

func (srv *Server) renderOrNotFound(w http.ResponseWriter, r *http.Request, status int, fileName ...string) {
	for _, fileName := range fileName {
		fileName = filepath.Join("www", fileName)
		if f, err := os.Open(fileName); !os.IsNotExist(err) {
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				defer f.Close()
				if pos := strings.LastIndexByte(fileName, '.'); pos != -1 {
					typ := mime.TypeByExtension(fileName[pos+1:])
					if typ == "" {
						typ = "text/html"
					}
					w.Header().Set("Content-Type", typ)
				}
				io.Copy(w, f)
			}
			return
		} else if tmpl, err := template.ParseFiles(fileName + ".tmpl"); !os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(status)
			if err == nil {
				err = tmpl.Execute(w, r)
			}
			if err != nil {
				w.Write([]byte(err.Error()))
			}
			return
		} else if f, err := os.Open(fileName + ".html"); !os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(status)
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				defer f.Close()
				io.Copy(w, f)
			}
			return
		}
	}
	http.NotFound(w, r)
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := r.URL.String()
	log.Println("HTTP:", r.Host, r.Proto, "<"+r.RemoteAddr+">", "`"+url+"`", "connected")
	defer func() {
		code := reflect.ValueOf(w).Elem().FieldByName("status").Int()
		log.Println("HTTP:", r.Host, "<"+r.RemoteAddr+">", code, "`"+url+"`", "done")
	}()

	if r.RequestURI == "/" || r.RequestURI == "" {
		srv.renderOrNotFound(w, r, 200, "index")
		return
	}

	if isWebsocketRequest(r) {
		srv.serveLocal(w, r)
		return
	}
	var (
		lb *LB

		host     = strings.SplitN(r.Host, ":", 2)[0]
		uriSlash = r.RequestURI
	)

	if !strings.HasSuffix(uriSlash, "/") {
		uriSlash += "/"
	}

	if pths, ok := srv.HttpHosts.Get(host); ok {
		for _, pth := range pths.sorted {
			if strings.HasPrefix(r.RequestURI, pth) {
				if lb_, ok := pths.Get(pth); ok {
					lb = lb_
					break
				}
			} else if uriSlash == pth {
				if lb_, ok := pths.Get(pth); ok {
					lb = lb_
					break
				}
			}
		}
	}

	if lb == nil {
		srv.renderOrNotFound(w, r, 404, r.RequestURI, "not_found", "index")
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
		close()
	}()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	n, err := io.Copy(&flushWriter{w}, &ner{resp.Body})

	prfx := fmt.Sprintf("[%s{%s}@%s]", lb.Ap, lb.Service, r.RemoteAddr)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		log.Println(prfx, "error:", err.Error())
	} else {
		log.Println(prfx, "transfered", n, "bytes")
	}
}

type ner struct {
	r io.Reader
}

func (t *ner) Read(p []byte) (n int, err error) {
	if n, err = t.r.Read(p); err != nil {
		return
	} else if n == 0 {
		err = io.EOF
		return
	}
	return
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
	if r.Header.Get(httpu.DefaultUriPrefixHeader) == "" && lb.HttpPath != "" && lb.HttpPath != "/" {
		outr.Header.Set(httpu.DefaultUriPrefixHeader, lb.HttpPath)
	}
	outr.RequestURI = ""

	return srv.proxyHTTP(lb, outr)
}

func (s *Server) proxyHTTP(lb *LB, r *http.Request) (resp *http.Response, err error) {
	var t http.RoundTripper
	if r.Proto == "HTTP/2" {
		var con net.Conn
		con, err = lb.Node.NextDial(nil, r.RemoteAddr)
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
				return lb.Node.NextDial(ctx, r.RemoteAddr)
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
