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
	"os"

	"github.com/moisespsena-go/xssh/server"
	"github.com/spf13/cobra"
)

var usersAddCmd = &cobra.Command{
	Use:   "add NAME...",
	Short: "Add one or more users",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var isAp, noUpdateKey bool
		if isAp, err = cmd.Flags().GetBool("ap"); err != nil {
			return
		}
		if noUpdateKey, err = cmd.Flags().GetBool("no-update-key"); err != nil {
			return
		}

		return withUsers(func(users *server.Users) (err error) {
			for i, name := range args {
				if err := users.Add(name, isAp, !noUpdateKey); err != nil {
					return fmt.Errorf("Add user %d %q failed: %v", i, name, err)
				} else {
					fmt.Fprintf(os.Stdout, "User %q added!\n", name)
				}
			}
			return nil
		})
	},
}

func init() {
	usersCmd.AddCommand(usersAddCmd)
	usersAddCmd.Flags().BoolP("ap", "A", false, "User is access point")
	usersAddCmd.Flags().BoolP("no-update-key", "K", false, "Disable auto update key")
}
