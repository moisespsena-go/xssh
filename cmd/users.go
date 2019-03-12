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
	"github.com/moisespsena-go/xssh/server"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Users manager",
}

func withDB(f func(DB *server.DB) error) error {
	DB := server.NewDB(dbName)
	DB.Init()
	defer DB.Close()
	return f(DB)
}

func withUsers(f func(users *server.Users) error) error {
	return withDB(func(DB *server.DB) error {
		users := server.NewUsers(DB)
		return f(users)
	})
}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.PersistentFlags().StringVar(&dbName, "db", dbName, "SQLite 3 database file")
}
