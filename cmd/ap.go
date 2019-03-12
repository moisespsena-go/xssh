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
	"strings"
	"sync"
	"time"

	"github.com/moisespsena-go/xssh/ap"
	"github.com/moisespsena-go/xssh/common"
	"github.com/spf13/cobra"
)

const defaultReconnectTimeout = "15s"

var apCmd = &cobra.Command{
	Use:   "ap NAME SERVICE...",
	Short: "X-SSH Access Point",
	Long: `X-SSH Access Point

# SERVICE

Pair or NAME/ADDR.

Example:
- SSH/localhost:22 (for ssh service, use in UPPER CASE)
- http/192.168.1.5:80
- https/192.168.1.5:443
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			connectionsCount int
			serverAddr       string
			sshAddr          string
			reconnectTimeout string
		)

		if connectionsCount, err = cmd.Flags().GetInt("connections-count"); err != nil {
			return
		}
		if connectionsCount < 1 {
			connectionsCount = 1
		}
		if serverAddr, err = cmd.Flags().GetString("server-addr"); err != nil {
			return
		}
		if sshAddr, err = cmd.Flags().GetString("ssh-addr"); err != nil {
			return
		}
		if reconnectTimeout, err = cmd.Flags().GetString("reconnect-timeout"); err != nil {
			return
		}
		var d time.Duration
		if d, err = time.ParseDuration(reconnectTimeout); err != nil {
			return fmt.Errorf("bad reconnect-timeout value: %v", err)
		}

		if d < time.Second {
			return fmt.Errorf("bad reconnect-timeout value: minimum value is `1s` (one second)")
		}

		var services = map[string]*ap.Service{}

		for _, service := range args[1:] {
			parts := strings.Split(service, "/")
			if parts[0] == "ssh" {
				parts[0] = strings.ToUpper(parts[0])
			}
			srvc := &ap.Service{Name: parts[0], Addr: parts[1]}
			log.Println(fmt.Sprintf("Service %q -> %s", parts[0], parts[1]))
			services[parts[0]] = srvc
		}

		/*
		ssh := ap.SSHServer(keyFile, sshAddr)
		services["ssh"] = ssh
		*/
		fmt.Sprint(sshAddr)

		var wg sync.WaitGroup
		wg.Add(connectionsCount)

		for i := 0; i <= connectionsCount; i++ {
			Ap := ap.New(args[0])
			Ap.ID = fmt.Sprintf("C%02d", i+1)
			Ap.Services = services
			Ap.KeyFile = keyFile
			Ap.ServerAddr = serverAddr
			Ap.SetReconnectTimeout(d)

			go func() {
				defer Ap.Close()
				defer wg.Done()
				Ap.Forever()
			}()
		}

		wg.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(apCmd)
	apCmd.Flags().IntP("connections-count", "C", 1, "Number of connections. Minimum is `1`.")
	apCmd.Flags().StringP("server-addr", "S", common.DefaultServerAddr, "The server addr")
	apCmd.Flags().StringP("ssh-addr", "A", common.DefaultApAddr, "The embeded SSH server addr")
	apCmd.Flags().StringP("reconnect-timeout", "T", defaultReconnectTimeout, reconnectTimeoutUsage)
}

const reconnectTimeoutUsage = `Reconnect to server timeout.
The value is a possibly signed sequence of decimal numbers,
each with optional fraction and a unit suffix, such as 
"10s", "1.5h" or "2h45m". Valid time units are "s" (second), 
"m" (minute), "h" (hour)`
