package server

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/go-errors/errors"

	"github.com/gliderlabs/ssh"
)

type ContextKey int

const (
	UnRegisterContextKey ContextKey = iota
)

type ServiceListener struct {
	Name string
	Listener
	Client ssh.Session
	cl     *ClientListeners
	node   *Node
}

func (sl *ServiceListener) Close() error {
	if sl.node != nil {
		defer sl.node.CloseEndPont(sl.Addr().String())
	}
	return sl.Listener.Close()
}

func (sl *ServiceListener) Release() {
	sl.cl.mu.Lock()
	defer sl.cl.mu.Unlock()
	sl.cl.count--
}

func (sl *ServiceListener) Lock() {
	sl.cl.mu.Lock()
	defer sl.cl.mu.Unlock()
	sl.cl.count++
}

type DefaultReversePortForwardingRegister struct {
	forwards  map[string]map[string]*ClientListeners
	mu        sync.Mutex
	Nodes     *Nodes
	HttpHosts *HttpHosts
}

func (r *DefaultReversePortForwardingRegister) Register(ctx ssh.Context, addr string, ln net.Listener) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var n *Node

	apName := ctx.User()
	clientKey := ctx.RemoteAddr().String()

	if v := ctx.Value(UnRegisterContextKey); v == nil {
		ctx.SetValue(UnRegisterContextKey, UnRegisterContextKey)
		cl := ctx.Value(ssh.ContextKeyCloseListener).(ssh.CloseListener)
		cl.CloseCallback(func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			_, ok := r.forwards[apName]
			if !ok {
				return
			}

			_, ok = r.forwards[apName][clientKey]
			if !ok {
				return
			}

			defer func() {
				log.Println("AP ", apName, "at", clientKey, " closed.")
				delete(r.forwards[apName], clientKey)

				if len(r.forwards[apName]) == 0 {
					delete(r.forwards, apName)
				}
			}()

			r.forwards[apName][clientKey].Close()
		})
	}

	if r.forwards == nil {
		r.forwards = map[string]map[string]*ClientListeners{}
	}
	_, ok := r.forwards[apName]
	if !ok {
		r.forwards[apName] = map[string]*ClientListeners{}
	}

	_, ok = r.forwards[apName][clientKey]
	if !ok {
		r.forwards[apName][clientKey] = &ClientListeners{}
	}
	serviceName := strings.TrimPrefix(strings.TrimPrefix(addr, "unix:"), "virtual:")

	sl := &ServiceListener{
		Listener: ln.(Listener),
		Name:     serviceName,
		cl:       r.forwards[apName][clientKey],
	}

	if serviceName[0] == '*' {
		sl.Name = sl.Name[1:]
		lb := ctx.Value("load_balancer:" + serviceName[1:]).(*LoadBalancer)
		var err error
		n, err = r.Nodes.Add(lb, sl)
		if err != nil {
			return errors.New("AP " + apName + "at" + clientKey + ": add endpoint to node failed:" + err.Error())
		}

		if lb.HttpHost != nil && len(n.EndPoints) == 1 {
			r.HttpHosts.Register(*lb.HttpHost).Set(lb, n.ChanListener.Dial)
			n.OnClose(func() {
				r.HttpHosts.Remove(*lb.HttpHost, lb.HttpPath)
			})
		}

		sl.node = n
	}

	r.forwards[apName][clientKey].Add(sl)
	return nil
}

func (r *DefaultReversePortForwardingRegister) UnRegister(ctx ssh.Context, addr string) (ln net.Listener, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user := ctx.User()
	_, ok2 := r.forwards[user]
	if !ok {
		return
	}

	clientKey := ctx.RemoteAddr().String()
	_, ok2 = r.forwards[user][clientKey]
	if !ok2 {
		return
	}
	r.forwards[user][clientKey].Remove(addr)

	if len(r.forwards[user][clientKey].byAddr) == 0 {
		delete(r.forwards, clientKey)
	}
	if len(r.forwards[user]) == 0 {
		delete(r.forwards, clientKey)
	}
	return
}

func (r *DefaultReversePortForwardingRegister) Get(ctx ssh.Context, addr string) (ln net.Listener, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := ctx.User()
	_, ok2 := r.forwards[user]
	if !ok {
		return
	}
	clientKey := ctx.RemoteAddr().String()
	_, ok2 = r.forwards[user][clientKey]
	if !ok2 {
		return
	}
	ln, ok = r.forwards[user][clientKey].byAddr[addr]
	return
}

func (r *DefaultReversePortForwardingRegister) GetListener(apName, serviceName string, addr ...string) (ln *ServiceListener, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	listeners, ok := r.forwards[apName]
	if !ok {
		err = fmt.Errorf("Ap %q not registered", apName)
		return
	}

	if len(addr) > 0 && addr[0] != "" {
		if apLns, ok := listeners[addr[0]]; ok {
			if ln, ok = apLns.byName[serviceName]; ok {
				return
			}
		}
		err = fmt.Errorf("Service %q not registered", serviceName)
		return
	}

	var lns []*ServiceListener

	for _, apLns := range listeners {
		if ln, ok = apLns.byName[serviceName]; ok {
			if ln.node != nil {
				return
			}
			lns = append(lns, ln)
		}
	}

	if len(lns) == 0 {
		err = fmt.Errorf("Service %q not registered", serviceName)
		return
	}

	sort.Slice(lns, func(i, j int) bool {
		return lns[i].cl.count < lns[j].cl.count
	})
	return lns[0], nil
}
