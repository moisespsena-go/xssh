package ap

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

type ServiceConfig struct {
	Name             string
	SocketPath       string
	NetAddr          string
	ConnectionsCount int
}

func (cfg ServiceConfig) String() (s string) {
	s += cfg.Name + "/"
	if cfg.SocketPath != "" {
		s += "unix:" + cfg.SocketPath
	} else {
		s += cfg.NetAddr
	}
	s += "/" + strconv.Itoa(cfg.ConnectionsCount)
	return
}

func ParseServiceDSN(dsn string) (cfg ServiceConfig, err error) {
	if dsn == "" {
		err = errors.New("empty")
		return
	}
	var parts = strings.Split(dsn, "/")
	if len(parts) <= 1 || len(parts) > 3 {
		err = errors.New("bad format")
		return
	}

	name, addr := parts[0], parts[1]
	if name == "" {
		err = errors.New("SERVICE_NAME is empty")
		return
	}
	if addr == "" {
		err = errors.New("ADDR is empty")
		return
	}
	cfg.Name = name
	if strings.HasPrefix(addr, "unix:") {
		cfg.SocketPath = strings.TrimPrefix(addr, "unix:")
	} else if host, port, err := net.SplitHostPort(addr); err != nil {
		return cfg, err
	} else {
		if host == "lo" {
			host = "localhost"
		}
		cfg.NetAddr = net.JoinHostPort(host, port)
	}

	if len(parts) == 3 {
		var ccount int
		if ccount, err = strconv.Atoi(parts[2]); err != nil {
			return
		}
		cfg.ConnectionsCount = ccount
	}

	return
}
