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
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")
		updaterCmd, _ := cmd.Flags().GetString("updater-cmd")
		updaterAddr, _ := cmd.Flags().GetString("updater-addr")

		if addr == "" {
			addr = common.DefaultServerPublicAddr
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

		return withDB(func(DB *server.DB) error {
			s := server.Server{
				Updater:        Updater,
				SocketsDir:     "sockets",
				KeyFile:        keyFile,
				Addr:           addr,
				Users:          server.NewUsers(DB),
				LoadBalancers:  server.NewLoadBalancers(DB),
				NodeSockerPerm: 0666,
			}

			s.Serve()
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringP("addr", "a", common.DefaultServerPublicAddr, "Public addr (default is `"+common.DefaultServerPublicAddr+"`).")
	serveCmd.Flags().String("updater-cmd", "", "Updater command")
	serveCmd.Flags().String("updater-addr", "", "Updater Addr")
	serveCmd.PersistentFlags().StringVar(&dbName, "db", dbName, "SQLite 3 database file")
}
