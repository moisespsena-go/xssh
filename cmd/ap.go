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
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	defaultlogger "github.com/moisespsena/go-default-logger"
	"github.com/robfig/cron"

	"github.com/moisespsena-go/xssh/updater"

	"github.com/moisespsena-go/task"
	"github.com/moisespsena-go/task/restarts"

	"github.com/moisespsena-go/xssh/ap"
	"github.com/moisespsena-go/xssh/common"
	"github.com/spf13/cobra"
)

const defaultReconnectTimeout = "15s"

var apCmd = &cobra.Command{
	Use:   "ap NAME[@SERVER_HOST] SERVICE_DSN...",
	Short: "X-SSH Access Point",
	Long: `X-SSH Access Point

# SERVICE_DSN

Pair or NAME/ADDR[/CONNECTION_COUNT].

Only TCP connections.

## NAME

The service name. For load balancer entry point use ` + q("*") + ` (asterisk) as prefix.

Single connections examples:
- http
- https
- SSH (for ssh service, use in UPPER CASE)
- my_service

Load balancer entry point examples:
- *http
- *https
- *SSH
- *my_service

## ADDR

If is unix socket path, use , other else,
use ` + q("") + ` or 

### Unix socket file

Format: ` + q("unix:PATH") + `.

Examples:
- unix:/path/to/sockfile.sock
- unix:/path/to/sockfile

### Network Address

The local service network address.

Format: ` + q("HOST:PORT") + `.

#### HOST
Format: ` + q("HOST_ADDR[%ZONE]") + ` 

*HOST_ADDR*: Accepts ` + q("IPv4") + ` or ` + q("IPv6") + ` or ` + q("HOST_NAME") + `.
For localhost, use ` + q("localhost") + ` or just ` + q("lo") + `.

*ZONE*: The zone. Example:  ` + q("%eth2") + `, ` + q("%wlan0") + `

Examples:
- 192.168.2.5
- 192.168.2.5%eth0
- [2001:db8::1%eth0]

#### Examples

- localhost:80
- lo:80
- 127.0.0.1:443
- 192.168.2.5:5000
- 192.168.2.5%wlan0:5000
- [2001:db8::1]:8080
- [2001:db8::1%eth0]:8080

## CONNECTION_COUNT

The connection count of service. This value is optional.
If is not defined, uses value of ` + q("connectiond-count") + ` flag.

## Complete examples

Default:
- SSH/lo:22
- SSH/localhost:22
- http/192.168.1.5%eth0:80
- https/192.168.1.5:443
- my_service/[2001:db8::1]:8080

With connection count:
- SSH/lo:22/6
- http/192.168.1.5:5%eth0:80/9
- https/192.168.1.5:443/4
- my_service/[2001:db8::1]:8080/2
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			user                   = args[0]
			connectionsCount, port int
			serverAddr, host       string
			reconnectTimeout       string
			updateInterval         string
			enableSSH              bool
		)

		args = args[1:]

		if enableSSH, err = cmd.Flags().GetBool("ssh"); err != nil {
			return
		}
		if connectionsCount, err = cmd.Flags().GetInt("connections-count"); err != nil {
			return
		}
		if port, err = cmd.Flags().GetInt("port"); err != nil {
			return
		}
		if serverAddr, err = cmd.Flags().GetString("server-addr"); err != nil {
			return
		}
		if host, err = cmd.Flags().GetString("host"); err != nil {
			return
		}
		if reconnectTimeout, err = cmd.Flags().GetString("reconnect-timeout"); err != nil {
			return
		}
		if updateInterval, err = cmd.Flags().GetString("update-interval"); err != nil {
			return
		}

		if connectionsCount < 1 {
			connectionsCount = 1
		}

		if serverAddr != "" {
			if h, p, err := net.SplitHostPort(serverAddr); err != nil {
				return fmt.Errorf("bad `server-addr` flag value: %v", err)
			} else {
				host = h
				if port, err = strconv.Atoi(p); err != nil {
					return fmt.Errorf("bad `server-addr` flag PORT value: %v", err)
				}
			}
		}

		if port == 0 {
			port = 2220
		}

		if i := strings.IndexRune(user, '@'); i > 0 {
			user, host = user[0:i], user[i+1:]
			if i = strings.IndexRune(host, ':'); i > 0 {
				if port, err = strconv.Atoi(host[i+1:]); err != nil {
					return fmt.Errorf("Parse SERVER_PORT failed: %v", err)
				}
				host = host[0:i]
			}
		}

		if host == "" {
			host = "localhost"
		}

		serverAddr = net.JoinHostPort(host, strconv.Itoa(port))

		var d time.Duration
		if d, err = time.ParseDuration(reconnectTimeout); err != nil {
			return fmt.Errorf("bad reconnect-timeout value: %v", err)
		}

		if d < time.Second {
			return fmt.Errorf("bad reconnect-timeout value: minimum value is `1s` (one second)")
		}

		updateSchedule, err := cron.Parse(updateInterval)
		if err != nil {
			return fmt.Errorf("bad update interval: %v", err)
		}

		var services = map[string]*ap.Service{}
		var servicesConfig []ap.ServiceConfig

		if enableSSH {
			services["ssh"] = ap.SSHServer(keyFile)
			servicesConfig = append(servicesConfig, ap.ServiceConfig{
				Name:             "ssh",
				ConnectionsCount: 1,
			})
		}

		for i, dsn := range args {
			cfg, err := ap.ParseServiceDSN(dsn)
			if err != nil {
				return fmt.Errorf("Parse SERVICE_DSN[%d] `%v` failed: %v", i, dsn, err)
			}

			if cfg.ConnectionsCount == 0 || cfg.ConnectionsCount > connectionsCount {
				cfg.ConnectionsCount = connectionsCount
			}

			if _, ok := services[cfg.Name]; ok {
				return fmt.Errorf("Parse SERVICE_DSN[%d] `%v` failed: service has be registered", i, dsn)
			}

			servicesConfig = append(servicesConfig, cfg)

			var addr string
			if cfg.SocketPath != "" {
				addr = "unix:" + cfg.SocketPath
			} else {
				addr = cfg.NetAddr
			}
			srvc := &ap.Service{Name: cfg.Name, Addr: addr}
			log.Println(fmt.Sprintf("Service `%v` -> `%s`", dsn, cfg))
			services[cfg.Name] = srvc
		}

		if len(services) == 0 {
			return fmt.Errorf("No services")
		}

		var maxCc = servicesConfig[0].ConnectionsCount

		for _, cfg := range servicesConfig[1:] {
			if cfg.ConnectionsCount >= maxCc {
				maxCc = cfg.ConnectionsCount
			}
		}

		if exe, err := os.Executable(); err == nil {
			Version.Digest, _ = common.Digest(exe)
		}
		// factory and stop chan

		t := task.FactoryFunc(func() task.Task {
			done := make(chan interface{})
			return task.NewTask(func() (err error) {
				for i := 1; i <= maxCc; i++ {
					Ap := ap.New(user)
					if i == i {
						Ap.Version = &Version
					}
					Ap.ID = fmt.Sprintf("C%02d", i)
					Ap.Services = map[string]*ap.Service{}

					for _, cfg := range servicesConfig {
						if cfg.ConnectionsCount >= i {
							Ap.Services[cfg.Name] = services[cfg.Name]
						}
					}

					Ap.KeyFile = keyFile
					Ap.ServerAddr = serverAddr
					Ap.SetReconnectTimeout(d)

					go func() {
						defer Ap.Close()
						Ap.Forever()
					}()
				}

				<-done
				return nil
			}, func() {
				close(done)
			})
		})

		restarts.RunConfig(
			restarts.New(t).SetLog(defaultlogger.NewLogger(os.Args[0])),
			&restarts.Config{
				FetchCronSchedule: &updateSchedule,
				Fetcher: &updater.Fetcher{
					ServerAddr: serverAddr,
					KeyFile:    keyFile,
					User:       user,
				},
			},
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(apCmd)

	apCmd.Flags().StringP("update-interval", "U", "@every 3s", "Update check interval. This is a cron Spec.")
	apCmd.Flags().IntP("port", "p", 2220, "XSSH server port.")
	apCmd.Flags().StringP("host", "H", "localhost", "SERVER_HOST: The XSSH server host.")
	apCmd.Flags().IntP("connections-count", "C", 1, "Number of connections. Minimum is `1`.")
	apCmd.Flags().Bool("ssh", false, "Enable embeded SSH server")
	apCmd.Flags().StringP("server-addr", "S", common.DefaultServerAddr, "The XSSH server addr in `HOST:PORT` format.")
	apCmd.Flags().StringP("reconnect-timeout", "T", defaultReconnectTimeout, reconnectTimeoutUsage)
}

const reconnectTimeoutUsage = `Reconnect to server timeout.
The value is a possibly signed sequence of decimal numbers,
each with optional fraction and a unit suffix, such as 
"10s", "1.5h" or "2h45m". Valid time units are "s" (second), 
"m" (minute), "h" (hour).`
