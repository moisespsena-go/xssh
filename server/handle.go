package server

import (
	"fmt"
	"github.com/moisespsena-go/xssh/common"
	"io"
	"log"
	"net"
	"reflect"
	"unsafe"

	"golang.org/x/crypto/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type session struct {
	conn    *ssh.ServerConn
	session *ssh.Session
	client  *ssh.Client
}

func (s *session) Close() error {
	if s.session != nil {
		s.session.Close()
	}
	if s.client != nil {
		s.client.Close()
	}
	return s.conn.Close()
}

func (sess *session) handle(preRequests []*ssh.Request, reqs <-chan *ssh.Request) {
	s := sess.session
	for _, req := range preRequests {
		if _, err := s.SendRequest(req.Type, req.WantReply, req.Payload); err != nil {
			log.Println(err)
			return
		}
	}

	if err := s.Shell(); err != nil {
		log.Println("SSH Shell:", err)
		return
	}
}

func (srv *Server) newSession(apName string, conn *ssh.ServerConn) (s *session, ok bool) {
	s = &session{conn: conn}
	apLn, err := srv.register.GetListener(apName, common.SrvcSSH)

	if err != nil {
		log.Printf("GetApListener: %v\n", err)
		return
	}

	sshConfig := &gossh.ClientConfig{
		User:            s.conn.User(),
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}

	s.client, err = gossh.Dial("tcp", apLn.Addr().String(), sshConfig)
	if err != nil {
		log.Printf("SSH Dial: %v\n", err)
		return
	}
	s.session, err = s.client.NewSession()
	if err != nil {
		log.Printf("SSH NewSession: %v\n", err)
		return
	}
	ok = true
	return
}

func netConOf(conn ssh.Conn) net.Conn {
	rf := reflect.ValueOf(conn).Elem().FieldByName("sshConn")
	rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
	rf = rf.FieldByName("conn")
	rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
	return rf.Interface().(net.Conn)
}

func (s *Server) handle(apName string, conn *ssh.ServerConn) {
	sess, ok := s.newSession(apName, conn)
	defer conn.Close()
	if !ok {
		return
	}

	remoteCon := netConOf(sess.client.Conn)
	clientConn := netConOf(conn.Conn)
	go io.Copy(clientConn, remoteCon)
	io.Copy(remoteCon, clientConn)


	fmt.Sprint(sess)
}
