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

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show users",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			nameMatch string
			isAp      bool
		)
		if nameMatch, err = cmd.Flags().GetString("name-match"); err != nil {
			return
		}
		if isAp, err = cmd.Flags().GetBool("ap"); err != nil {
			return
		}
		return withUsers(func(users *server.Users) error {
			var count int
			err := users.List(isAp, func(i int, u *server.User) error {
				count = i
				fmt.Fprintln(os.Stdout, i, "\t", u)
				return nil
			}, nameMatch)
			if err != nil {
				return err
			}
			if count == 0 {
				fmt.Fprintln(os.Stdout, "No users found.")
			} else {
				fmt.Fprintf(os.Stdout, "\n%d users found.\n", count)
			}
			return nil
		})
	},
}

func init() {
	usersCmd.AddCommand(usersListCmd)
	usersListCmd.Flags().BoolP("ap", "A", false, "List access points")
	usersListCmd.Flags().StringP("name-match", "N", "", "User name match criteria")
}
