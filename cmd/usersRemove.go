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

var usersRemoveCmd = &cobra.Command{
	Use:   "remove NAME...",
	Short: "Remove one or more users",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withUsers(func(users *server.Users) (err error) {
			if count, err := users.Remove(args...); err != nil {
				return fmt.Errorf("Remove users %s failed: %v", args, err)
			} else if count == 0 {
				fmt.Fprintln(os.Stdout, "No users removed!")
			} else {
				fmt.Fprintln(os.Stdout, count, "users removed!")
			}
			return nil
		})
	},
}

func init() {
	usersCmd.AddCommand(usersRemoveCmd)
}
