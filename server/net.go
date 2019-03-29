package server

import (
	"context"
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
	ProtoAddr() string
}

type UnixListener struct {
	net.Listener
	Str        string
	RootDir    string
	SocketPath string
	SockPerm   os.FileMode
	onClose    []func()
}

func (l *UnixListener) ProtoAddr() string {
	return "unix:" + l.SocketPath
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

func (l AddrListener) ProtoAddr() string {
	if l.str == "" {
		return l.ProtoAddr()
	}
	return l.str
}

func (l AddrListener) String() string {
	return l.StrPrefix + l.ProtoAddr()
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
	addr    VirtualAddr
	src     chan net.Conn
	onClose []func()
	mu      sync.Mutex
}

func (l *ChanListener) String() string {
	return l.addr.String()
}

func (l *ChanListener) Listen() (err error) {
	if l.src != nil {
		return errors.New(l.ProtoAddr() + " is listening")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.src != nil {
		return errors.New(l.ProtoAddr() + " is listening")
	}
	l.src = make(chan net.Conn)
	return nil
}

func NewChanListener(name string) *ChanListener {
	return &ChanListener{addr: VirtualAddr{name}}
}

func (l *ChanListener) ProtoAddr() string {
	return l.addr.String()
}

func (l *ChanListener) OnClose(f ...func()) {
	l.onClose = append(l.onClose, f...)
}

func (l *ChanListener) Accept() (net.Conn, error) {
	if l.src == nil {
		return nil, io.EOF
	}
	con := <-l.src
	if con == nil {
		return nil, io.EOF
	}
	if l.src == nil {
		con.Close()
		return nil, io.EOF
	}
	return con, nil
}

func (l *ChanListener) Close() error {
	if l.src == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.src == nil {
		return nil
	}
	close(l.src)
	l.src = nil
	for _, cb := range l.onClose {
		cb()
	}
	return nil
}

func (l *ChanListener) Addr() (addr net.Addr) {
	return l.addr
}

func (l *ChanListener) Dial(ctx context.Context, remoteAddr string) (con net.Conn, err error) {
	laddr, radd := &VirtualAddr{remoteAddr}, &VirtualAddr{"{" + remoteAddr + "->" + l.addr.Name + "}"}
	ir, iw := io.Pipe()
	or, ow := io.Pipe()

	rConn := &VirtualCon{Writer: ow, Reader: ir, RAddr: radd, LAddr: laddr}
	l.src <- rConn
	lConn := &VirtualCon{Writer: iw, Reader: or, RAddr: laddr, LAddr: radd}
	return lConn, nil
}

type VirtualAddr struct {
	Name string
}

func (addr VirtualAddr) Network() string {
	return "<nil>"
}

func (addr VirtualAddr) String() string {
	return "virtual:" + addr.Name
}

type VirtualCon struct {
	Writer io.Writer
	Reader io.Reader
	LAddr  net.Addr
	RAddr  net.Addr
	mu     sync.Mutex
	closed bool
}

func (con VirtualCon) Write(p []byte) (n int, err error) {
	if con.closed {
		err = io.EOF
		return
	}
	return con.Writer.Write(p)
}

func (con VirtualCon) Read(p []byte) (n int, err error) {
	if con.closed {
		err = io.EOF
		return
	}
	return con.Reader.Read(p)
}

func (con VirtualCon) LocalAddr() net.Addr {
	return con.LAddr
}

func (con VirtualCon) RemoteAddr() net.Addr {
	return con.RAddr
}

func (con VirtualCon) SetDeadline(t time.Time) error {
	return nil
}

func (con VirtualCon) SetReadDeadline(t time.Time) error {
	return nil
}

func (con VirtualCon) SetWriteDeadline(t time.Time) error {
	return nil
}

func (con VirtualCon) Close() error {
	if con.closed {
		return nil
	}
	con.mu.Lock()
	defer con.mu.Unlock()
	if con.closed {
		return nil
	}
	con.closed = true

	if closer, ok := con.Writer.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := con.Reader.(io.Closer); ok {
		closer.Close()
	}
	return nil
}
