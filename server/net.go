package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/phayes/permbits"

	"github.com/moisespsena-go/xssh/common"
)

type Listener interface {
	net.Listener
	fmt.Stringer

	Listen() (err error)
}

type UnixListener struct {
	net.Listener
	Str        string
	RootDir    string
	SocketPath string
	SockPerm   os.FileMode
	onClose    []func()
}

func (l *UnixListener) OnClose(f ...func()) {
	l.onClose = append(l.onClose, f...)
}

func (l UnixListener) String() string {
	return l.Str
}

func (l *UnixListener) Listen() (err error) {
	if _, err2 := os.Stat(l.SocketPath); err2 == nil {
		if err = l.RemoveSocketFile(); err != nil {
			return
		}
	}
	d := filepath.Dir(l.SocketPath)
	if _, err2 := os.Stat(d); os.IsNotExist(err2) {
		perms := permbits.FileMode(l.SockPerm)
		perms.SetUserExecute(true)
		if perms.GroupRead() {
			perms.SetGroupExecute(true)
		}
		if perms.OtherExecute() {
			perms.SetOtherExecute(true)
		}
		if err = os.MkdirAll(d, os.FileMode(perms)); err != nil {
			return
		}
	}
	l.Listener, err = net.Listen("unix", l.SocketPath)
	if err != nil {
		err = errors.New(l.String() + " listen failed: " + err.Error())
		return
	}
	if l.SockPerm != 0 {
		if err = os.Chmod(l.SocketPath, l.SockPerm); err != nil {
			l.Close()
			l.Listener = nil
		}
	}
	return
}

func (l *UnixListener) RemoveSocketFile() (err error) {
	if err = os.Remove(l.SocketPath); err != nil {
		log.Println(l.String(), "remove socket file `"+l.SocketPath+"` failed:", err.Error())
	}
	return
}

func (l *UnixListener) Addr() net.Addr {
	if l.Listener == nil {
		return &net.UnixAddr{Net: "unix", Name: l.SocketPath}
	}
	return l.Listener.Addr()
}

func (l *UnixListener) Close() (err error) {
	defer func() {
		for _, f := range l.onClose {
			f()
		}
	}()
	if l.Listener != nil {
		defer log.Println(l.String(), "Closed")
		if err = l.Listener.Close(); err != nil {
			log.Println(l.String(), "close failed:", err.Error())
		}
		l.Listener = nil
		if err == nil {
			if err = common.RemoveEmptyDir(l.RootDir, filepath.Dir(l.SocketPath)); err != nil {
				log.Println(l.String(), err.Error())
			}
		}
	}
	return
}

type AddrListener struct {
	AddrS string
	net.Listener
	StrPrefix string
	str       string
}

func (l AddrListener) String() string {
	if l.str == "" {
		return l.StrPrefix + l.AddrS
	}
	return l.StrPrefix + l.str
}

func (l *AddrListener) Listen() (err error) {
	l.Listener, err = net.Listen("tcp", l.AddrS)
	if err != nil {
		err = errors.New(l.String() + " listen failed: " + err.Error())
	}
	l.str = l.Listener.Addr().String()
	return
}

func (l *AddrListener) Close() (err error) {
	if l.Listener != nil {
		defer log.Println(l.String(), "Closed")
		err = l.Listener.Close()
		return
	}
	return nil
}

type ChanListener struct {
	addr    ChanListenerAddr
	Source  chan net.Conn
	onClose []func()
	mu      sync.Mutex
}

func NewChanListener(addr ChanListenerAddr, source chan net.Conn) *ChanListener {
	if source == nil {
		source = make(chan net.Conn)
	}
	return &ChanListener{addr: addr, Source: source}
}

func (l *ChanListener) OnClose(f ...func()) {
	l.onClose = append(l.onClose, f...)
}

func (l *ChanListener) Accept() (net.Conn, error) {
	if l.Source == nil {
		return nil, io.EOF
	}
	con := <-l.Source
	if con == nil {
		return nil, io.EOF
	}
	if l.Source == nil {
		con.Close()
		return nil, io.EOF
	}
	return con, nil
}

func (l *ChanListener) Close() error {
	if l.Source == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Source == nil {
		return nil
	}
	close(l.Source)
	l.Source = nil
	for _, cb := range l.onClose {
		cb()
	}
	return nil
}

func (l *ChanListener) Addr() (addr net.Addr) {
	return l.addr
}

type ChanListenerAddr struct {
	Name string
}

func (addr ChanListenerAddr) Network() string {
	return "<nil>"
}

func (addr ChanListenerAddr) String() string {
	return "chan:" + addr.Name
}

type NetCon struct {
	io.Writer
	io.Reader
	LAddr net.Addr
	RAddr net.Addr
}

func (con NetCon) LocalAddr() net.Addr {
	return con.LAddr
}

func (con NetCon) RemoteAddr() net.Addr {
	return con.RAddr
}

func (con NetCon) SetDeadline(t time.Time) error {
	return nil
}

func (con NetCon) SetReadDeadline(t time.Time) error {
	return nil
}

func (con NetCon) SetWriteDeadline(t time.Time) error {
	return nil
}

func (con NetCon) Close() error {
	if closer, ok := con.Writer.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := con.Reader.(io.Closer); ok {
		closer.Close()
	}
	return nil
}
