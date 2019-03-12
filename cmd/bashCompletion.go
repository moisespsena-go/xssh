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
	"path/filepath"
	"strings"

	"github.com/moisespsena-go/xssh/common"

	"github.com/spf13/cobra"
)

var binRelHome = brh()

func brh() string {
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("Get executable path failed: %v", err))
	}

	d := filepath.Dir(exe)

	for _, pth := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if pth == d {
			return filepath.Base(exe)
		}
	}

	s, err := filepath.Rel(common.CurrentUser.HomeDir, exe)
	if err != nil {
		panic(fmt.Errorf("Get executable relative path to $HOME failed: %v", err))
	}

	return filepath.Join("$HOME/", s)
}

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generates bash completion scripts",
	Long: `To load completion run

. <(` + binRelHome + ` completion)

To configure your bash shell to load completions for each session add to your bashrc

# ~/.bashrc or ~/.profile
. <(` + binRelHome + ` completion)
`,
	Run: func(cmd *cobra.Command, args []string) {
		rootCmd.GenBashCompletion(os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
