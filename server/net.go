package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	path_helpers "github.com/moisespsena/go-path-helpers"

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
	SocketPath string
	SockPerm   os.FileMode
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
		if perm, err := path_helpers.ResolvPerms(d); err == nil {
			os.MkdirAll(d, os.FileMode(perm))
		}
	}
	l.Listener, err = net.Listen("unix", l.SocketPath)
	if err != nil {
		err = errors.New(l.String() + " listen failed: " + err.Error())
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

func (l *UnixListener) Close() (err error) {
	if l.Listener != nil {
		defer log.Println(l.String(), "Closed")
		if err = l.Listener.Close(); err != nil {
			log.Println(l.String(), "close failed:", err.Error())
		}
		l.Listener = nil
		if err == nil {
			if err = common.RemoveEmptyDir(filepath.Dir(l.SocketPath), 2); err != nil {
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
