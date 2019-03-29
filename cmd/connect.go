// Copyright © 2019 Moises P. Sena <moisespsena@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/moisespsena-go/xssh/server"
	"golang.org/x/crypto/ssh"

	"github.com/go-errors/errors"

	"github.com/moisespsena-go/httpu"

	"github.com/spf13/cobra"
	"golang.org/x/net/websocket"
)

func connect(done chan interface{}, tlsConfig *tls.Config, hostPort, token, ap, service string, rwc io.ReadWriteCloser) (closer io.Closer, err error) {
	var (
		server = "ws"
		origin = "http"
	)

	if tlsConfig != nil {
		server += "s"
		origin += "s"
	}

	server += "://" + hostPort
	origin += "://" + hostPort

	var h = make(http.Header)
	h.Set("X-Ap", ap)
	h.Set("X-Service", service)
	h.Set("Authorization", "Token "+token)

	cfg, _ := websocket.NewConfig(server, origin)
	cfg.Header = h
	cfg.Dialer = &net.Dialer{Timeout: 3 * time.Second}
	if tlsConfig != nil {
		cfg.TlsConfig = tlsConfig
	}

	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return nil, err
	}

	go func() {
		defer ws.Close()
		defer rwc.Close()
		io.Copy(ws, rwc)
	}()

	go func() {
		defer ws.Close()
		defer rwc.Close()
		defer close(done)
		io.Copy(rwc, ws)
	}()
	return ws, nil
}

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect SERVICE@AP",
	Args:  cobra.ExactArgs(1),
	Short: "Connect to AP using server TOKEN",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			addr, _          = cmd.Flags().GetString("addr")
			serverPort, _    = cmd.Flags().GetInt("port")
			host, _          = cmd.Flags().GetString("host")
			https, _         = cmd.Flags().GetBool("https")
			insecure, _      = cmd.Flags().GetBool("insecure")
			tokenFile, _     = cmd.Flags().GetString("token-file")
			token, _         = cmd.Flags().GetString("token")
			httpsCertFile, _ = cmd.Flags().GetString("https-cert-file")
			httpsKeyFile, _  = cmd.Flags().GetString("https-key-file")

			parts = strings.Split(args[0], "@")

			ap, service, hostPort string
			tlsConfig             *tls.Config
		)

		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return errors.New("bad SERVICE@AP value")
		}

		service, ap = parts[0], parts[1]

		if token == "" && tokenFile != "" {
			var b []byte
			if b, err = ioutil.ReadFile(tokenFile); err != nil {
				return
			}
			token = string(b)
		}

		if token == "" {
			return errors.New("token is empty")
		}

		if https {
			cer, err := tls.LoadX509KeyPair(httpsCertFile, httpsKeyFile)
			if err != nil {
				return err
			}

			tlsConfig = &tls.Config{
				InsecureSkipVerify: insecure,
				Certificates:       []tls.Certificate{cer},
			}
		}

		if serverPort == 0 {
			if https {
				serverPort = 2443
			} else {
				serverPort = 2080
			}
		}

		if host == "" {
			host = "localhost"
		}

		hostPort = fmt.Sprintf("%s:%d", host, serverPort)

		if addr == "" || addr == "-" {
			var (
				closer io.Closer
				done   = make(chan interface{})
			)

			if service == "ssh" {
				var (
					ir, iw = io.Pipe()
					or, ow = io.Pipe()
					rwc    = &rwcloser{r: or, w: iw}
				)

				closer, err = connect(done, tlsConfig, hostPort, token, ap, service, rwc)

				if err == nil {
					go func() {
						defer closer.Close()
						defer rwc.Close()
						var (
							client    *ssh.Client
							sshConfig = &ssh.ClientConfig{
								HostKeyCallback: ssh.InsecureIgnoreHostKey(),
							}
							caddr               = server.VirtualAddr{"pipe"}
							conn                = server.VirtualCon{Writer: ow, Reader: ir, LAddr: caddr, RAddr: caddr}
							c, chans, reqs, err = ssh.NewClientConn(conn, addr, sshConfig)
						)
						if err != nil {
							log.Fatal(err)
						}

						client = ssh.NewClient(c, chans, reqs)
						defer client.Close()

						s, err := client.NewSession()
						if err != nil {
							log.Println("create ssh session failed:", err.Error())
							return
						}

						defer s.Close()

						s.Stdout = os.Stdout
						s.Stderr = os.Stderr
						s.Stdin = os.Stdin

						modes := ssh.TerminalModes{
							ssh.ECHO:          1,      // please print what I type
							ssh.ECHOCTL:       0,      // please don't print control chars
							ssh.TTY_OP_ISPEED: 115200, // baud in
							ssh.TTY_OP_OSPEED: 115200, // baud out
						}

						termFD := int(os.Stdin.Fd())

						w, h, _ := terminal.GetSize(termFD)

						termState, _ := terminal.MakeRaw(termFD)
						defer terminal.Restore(termFD, termState)

						err = s.RequestPty("xterm-256color", h, w, modes)
						if err != nil {
							log.Println("request for pseudo terminal failed: %s", err)
							return
						}

						err = s.Shell()
						if err != nil {
							log.Println("failed to start shell: %s", err)
							return
						}
						err = s.Wait()
						if err != nil {
							log.Println("wait failed: %s", err)
						}
					}()
				}
			} else {
				closer, err = connect(done, tlsConfig, hostPort, token, ap, service, &rwcloser{w: os.Stdout, r: os.Stdin})
			}
			if err != nil {
				return
			}
			defer closer.Close()
			<-done
			return
		} else {
			Addr := httpu.Addr(addr)
			ln, err := Addr.CreateListener()
			if err != nil {
				return err
			}
			for {
				con, err := ln.Accept()
				if err != nil {
					return err
				}
				log.Println("new connection from:", con.RemoteAddr().String())
				go func(con net.Conn) {
					defer log.Println(con.RemoteAddr().String(), "closed")

					done := make(chan interface{})
					closer, err := connect(done, tlsConfig, hostPort, token, ap, service, con)
					if err != nil {
						log.Println("connect failed:", err.Error())
					}
					defer closer.Close()
					<-done
				}(con)
			}
			defer ln.Close()
			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)

	flags := connectCmd.Flags()

	flags.StringP("addr", "A", "-", "Local address. If value is `-`, use STDIN and STDOUT.")
	flags.IntP("port", "p", 0, "XSSH server HTTP port (default is `2443` If HTTPS else `2080`).")
	flags.StringP("host", "H", "localhost", "SERVER_HOST: The XSSH server host.")
	flags.Bool("https", false, "Connect with HTTPS")
	flags.Bool("insecure", false, "Allow HTTPS insecure hosts")
	flags.String("https-cert-file", "server.crf", "TLS cert file")
	flags.String("https-key-file", "server.key", "TLS key file")
	flags.String("token-file", "xssh.token", "The token file")
	flags.String("token", "", "The token")
}

type rwcloser struct {
	w      io.Writer
	r      io.Reader
	closed bool
}

func (r *rwcloser) Write(p []byte) (n int, err error) {
	if r.closed {
		err = io.ErrClosedPipe
		return
	}
	return r.w.Write(p)
}

func (r *rwcloser) Read(p []byte) (n int, err error) {
	if r.closed {
		err = io.ErrClosedPipe
		return
	}
	return r.r.Read(p)
}

func (r *rwcloser) Close() error {
	if closer, ok := r.w.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := r.r.(io.Closer); ok {
		closer.Close()
	}
	r.closed = true
	return nil
}
