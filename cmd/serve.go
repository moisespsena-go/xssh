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
	"github.com/robfig/cron"
	"os"

	"github.com/moisespsena-go/httpu"
	"github.com/moisespsena-go/overseer-task-restarts"
	"github.com/moisespsena-go/task"

	"github.com/anmitsu/go-shlex"
	"github.com/moisespsena-go/xssh/common"
	"github.com/moisespsena-go/xssh/server"
	"github.com/moisespsena-go/xssh/server/updater"
	"github.com/spf13/cobra"
)

var dbName = "xssh.db"

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "X-SSH The server",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		renewToken, _ := cmd.Flags().GetString("renew-token")
		addr, _ := cmd.Flags().GetString("addr")
		updaterCmd, _ := cmd.Flags().GetString("updater-cmd")
		updaterAddr, _ := cmd.Flags().GetString("updater-addr")
		httpKeepAlive, _ := cmd.Flags().GetString("http-keep-alive")
		httpKeepAliveIdle, _ := cmd.Flags().GetString("http-keep-alive-idle")
		httpKeepAliveCount, _ := cmd.Flags().GetInt("http-keep-alive-count")
		httpAddr, _ := cmd.Flags().GetString("http-addr")
		https, _ := cmd.Flags().GetBool("https")
		httpsAddr, _ := cmd.Flags().GetString("https-addr")
		httpsCertFile, _ := cmd.Flags().GetString("https-cert-file")
		httpsKeyFile, _ := cmd.Flags().GetString("https-key-file")
		httpsDisableHttp2, _ := cmd.Flags().GetBool("https-disable-http2")

		if addr == "" {
			addr = common.DefaultServerPublicAddr
		}

		renewTokenSchedule, err := cron.Parse(renewToken)
		if err != nil {
			return fmt.Errorf("bad token-renew flag value: %v", err)
		}

		var Updater updater.Updater

		if updaterCmd != "" {
			updaterCmdArgs, err := shlex.Split(updaterCmd, true)
			if err != nil {
				return fmt.Errorf("parse updater-cmd flag value: %v", err)
			}
			Updater = updater.NewCommandUpdater(updaterCmdArgs[0], updaterCmdArgs[1:]...)
		} else if updaterAddr != "" {
			Updater = updater.NewNetUpdater(updaterAddr)
		}

		if https {
			if _, err := os.Stat(httpsKeyFile); err != nil {
				return fmt.Errorf("`--https-key-file` flag: %v", err)
			}
			if _, err := os.Stat(httpsCertFile); err != nil {
				return fmt.Errorf("`--https-cert-file` flag: %v", err)
			}
		}

		var keepAliveConfig *httpu.KeepAliveConfig
		if httpKeepAlive != "" {
			keepAliveConfig = &httpu.KeepAliveConfig{Value: httpKeepAlive}
		}

		var keepAliveIdleConfig *httpu.KeepAliveConfig
		if httpKeepAliveIdle != "" {
			keepAliveIdleConfig = &httpu.KeepAliveConfig{Value: httpKeepAliveIdle}
		}

		var done func() error

		defer func() {
			if done != nil {
				done()
			}
		}()

		return restarts.New(task.FactoryFunc(func() task.Task {
			DB := server.NewDB(dbName).Init()
			done = DB.Close

			var httpConfig *httpu.Config
			if httpAddr != "" || (https && httpsAddr != "") {
				httpConfig = &httpu.Config{}
				if httpAddr != "" {
					httpConfig.Listeners = append(httpConfig.Listeners, httpu.ListenerConfig{
						KeepAliveInterval:     keepAliveConfig,
						KeepAliveIdleInterval: keepAliveIdleConfig,
						KeepAliveCount:        httpKeepAliveCount,
						Addr:                  httpu.Addr(httpAddr),
					})
				}
				if https && httpsAddr != "" {
					httpConfig.Listeners = append(httpConfig.Listeners, httpu.ListenerConfig{
						KeepAliveInterval:     keepAliveConfig,
						KeepAliveIdleInterval: keepAliveIdleConfig,
						KeepAliveCount:        httpKeepAliveCount,
						Addr:                  httpu.Addr(httpsAddr),
						Tls: httpu.TlsConfig{
							CertFile:    httpsCertFile,
							KeyFile:     httpsKeyFile,
							NPNDisabled: httpsDisableHttp2,
						},
					})
				}
			}

			return &server.Server{
				Updater:            Updater,
				SocketsDir:         "sockets",
				KeyFile:            keyFile,
				Addr:               addr,
				HttpConfig:         httpConfig,
				Users:              server.NewUsers(DB),
				LoadBalancers:      server.NewLoadBalancers(DB),
				NodeSockerPerm:     0666,
				RenewTokenSchedule: renewTokenSchedule,
			}
		})).RunWait()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	flags := serveCmd.Flags()

	// token
	flags.String("renew-token", "@daily", "Token renew interval. This is a cron Spec [see https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format].")
	// net
	flags.StringP("addr", "a", common.DefaultServerPublicAddr, "Public addr")
	// updater
	flags.String("updater-cmd", "", "Updater command")
	flags.String("updater-addr", "", "Updater Addr")
	// http server
	flags.String("http-addr", ":2080", "HTTP Addr")
	flags.String("http-keep-alive", "", httpKeepAliveUsage)
	flags.String("http-keep-alive-idle", "", httpKeepAliveIdleUsage)
	flags.Int("http-keep-alive-count", 0, "HTTP TCP Keep Alive count")
	// https server
	flags.Bool("https", false, "Enable HTTPS")
	flags.String("https-addr", ":2443", "HTTPS Addr")
	flags.String("https-cert-file", "server.crf", "TLS cert file")
	flags.String("https-key-file", "server.key", "TLS key file")
	flags.Bool("https-disable-http2", false, "Disable support for HTTP/2 protocol in HTTPS connections")

	serveCmd.PersistentFlags().StringVar(&dbName, "db", dbName, "SQLite 3 database file")
}

const httpKeepAliveUsage = `HTTP TCP Keep Alive duration.
The value is a possibly signed (seconds) or signed sequence of decimal numbers,
each with optional fraction and a unit suffix, such as 
"10" or "10s", "1.5h" or "2h45m". Valid time units are "s" (second), 
"m" (minute), "h" (hour).`

const httpKeepAliveIdleUsage = `HTTP TCP Keep Alive IDLE timeout.
The value is a possibly signed (seconds) or signed sequence of decimal numbers,
each with optional fraction and a unit suffix, such as 
"10" or "10s", "1.5h" or "2h45m". Valid time units are "s" (second), 
"m" (minute), "h" (hour).`
