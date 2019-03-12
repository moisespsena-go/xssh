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
	"os"

	"github.com/moisespsena-go/task/restarts"
	defaultlogger "github.com/moisespsena/go-default-logger"

	"github.com/moisespsena-go/xssh/forwarder"

	"github.com/moisespsena-go/xssh/common"

	"github.com/spf13/cobra"
)

var forwardCmd = &cobra.Command{
	Use:   "forward DSN SERVICE...",
	Short: "X-SSH Access Point connection forward",
	Long: `X-SSH Access Point connection forward
# DSN

DSN is [USER:]AP_NAME@XSSH_SERVER_HOST

# SERVICE

SERVICE is pair of name and local addr (NAME:ADDR).
If ADDR is empty, uses localhost:0.

Examples:
- 'ssh' eq 'ssh::2222' eq 'ssh:localhost:2222' (only ssh service has default port)
- 'ssh:domain.com:' eq 'ssh:domain.com:2222'
- 'a::7000' eq 'a:localhost:7000'
- 'a:domain.com:7000'
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			dsn              = args[0]
			port             int
			sshAddr          string
			reconnectTimeout string
			serviceNames     = args[1:]
			ssh              bool
		)

		if port, err = cmd.Flags().GetInt("port"); err != nil {
			return
		}
		if sshAddr, err = cmd.Flags().GetString("ssh-addr"); err != nil {
			return
		}
		if reconnectTimeout, err = cmd.Flags().GetString("reconnect-timeout"); err != nil {
			return
		}
		if ssh, err = cmd.Flags().GetBool("ssh"); err != nil {
			return
		}

		if ssh {
			serviceNames = append(serviceNames, "ssh:"+sshAddr)
		}

		c := &forwarder.Creator{
			ServiceNames:     serviceNames,
			ReconnectTimeout: reconnectTimeout,
			DSN:              dsn,
			KeyFile:          keyFile,
			Port:             port,
		}

		if t, err := c.Create(); err != nil {
			return err
		} else {
			restarts.Run(restarts.New(t).
				SetLog(defaultlogger.NewLogger(os.Args[0])))
			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(forwardCmd)
	forwardCmd.Flags().Bool("ssh", false, "forward SSH service")
	forwardCmd.Flags().IntP("port", "p", 2220, "XSSH server port")
	forwardCmd.Flags().StringP("ssh-addr", "A", common.DefaultClientAddr, "The access point embeded SSH server forward addr")
	forwardCmd.Flags().StringP("reconnect-timeout", "T", defaultReconnectTimeout, reconnectTimeoutUsage)
}
