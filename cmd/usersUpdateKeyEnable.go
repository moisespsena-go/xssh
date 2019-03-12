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

var usersUpdateKeyEnableCmd = &cobra.Command{
	Use:   "enable NAME...",
	Short: "Enable auto update key flag to one or more users",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		return withUsers(func(users *server.Users) (err error) {
			if count, err := users.SetUpdateKeyFlag(true, args...); err != nil {
				return fmt.Errorf("Enable auto update key flag failed: %v", args, err)
			} else if count == 0 {
				fmt.Fprintln(os.Stdout, "No users updated!")
			} else {
				fmt.Fprintln(os.Stdout, count, "users updated!")
			}
			return nil
		})
	},
}

func init() {
	usersUpdateKeyCmd.AddCommand(usersUpdateKeyEnableCmd)
}
