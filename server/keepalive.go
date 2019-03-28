package server

import (
	"crypto/tls"
	"net"
	"reflect"
	"time"
	"unsafe"

	"github.com/felixge/tcpkeepalive"
)

var (
	// DefaultKeepAliveIdleInterval specifies how long connection can be idle
	// before sending keepalive message.
	DefaultKeepAliveIdleInterval = 15 * time.Minute
	// DefaultKeepAliveCount specifies maximal number of keepalive messages
	// sent before marking connection as dead.
	DefaultKeepAliveCount = 8
	// DefaultKeepAliveInterval specifies how often retry sending keepalive
	// messages when no response is received.
	DefaultKeepAliveInterval = 5 * time.Second
)

func (srv *Server) keepAlive(conn net.Conn) error {
	if tlsConn, ok := conn.(*tls.Conn); ok {
		var t time.Time
		tlsConn.SetDeadline(t)
		tlsConn.SetWriteDeadline(t)
		v := reflect.ValueOf(tlsConn).Elem().FieldByName("conn")
		ptrToY := unsafe.Pointer(v.UnsafeAddr())
		realPtrToY := (*net.Conn)(ptrToY)
		conn = *(realPtrToY)
	}
	return tcpkeepalive.SetKeepAlive(conn, DefaultKeepAliveIdleInterval, DefaultKeepAliveCount, DefaultKeepAliveInterval)
}
