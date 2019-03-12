// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
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
	"github.com/moisespsena-go/task"
	"github.com/moisespsena-go/task/restarts"
	"github.com/moisespsena-go/xssh/forwarder"
	"github.com/moisespsena/go-default-logger"
	"net"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// forwardSshCmd represents the forwardSsh command
var forwardSshCmd = &cobra.Command{
	Use:   "ssh DSN [SSH args]...",
	Short: "Connect to fowarded SSH server",
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	Long: `
Connect to fowarded SSH server.

# DSN

DSN is [USER:]AP_NAME@XSSH_SERVER_HOST
`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var port int

		if port, err = cmd.Flags().GetInt("port"); err != nil {
			return
		}

		c := &forwarder.Creator{
			ServiceNames:     []string{"ssh:localhost:0"},
			ReconnectTimeout: "5s",
			DSN:              args[0],
			KeyFile:          keyFile,
			Port:             port,
		}

		if t, err := c.Create(); err != nil {
			return err
		} else {
			t2 := task.FactoryFunc(func() (task.Task) {
				args := append([]string{}, os.Args[3:]...)
				for i, l := 0, len(args); i < l; i++ {
					switch args[i]  {
					case "-p":
						_, p, _ := net.SplitHostPort(t.GetService("ssh").Addr)
						args[i+1] = fmt.Sprint(p)
						i++
					case c.DSN:
						args[i] = t.UserName + "@localhost"
					}

				}

				cmd := exec.Command("ssh", args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				cmd.Env = os.Environ()

				return task.NewCmdTask(cmd)
			})
			restarts.Run(restarts.New(t, t2).
				SetLog(defaultlogger.NewLogger(os.Args[0])))
			return nil
		}

	},
}

func init() {
	forwardCmd.AddCommand(forwardSshCmd)
	forwardSshCmd.Flags().IntP("port", "p", 2220, "XSSH server port")
}
