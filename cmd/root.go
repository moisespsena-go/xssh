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

	"github.com/moisespsena-go/xssh/common"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	keyFile = common.GetKeyFile()
)

var rootCmd = &cobra.Command{
	Use:   "xssh",
	Short: "X-SSH - The Extreme SSH tool",
}

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	//cobra.OnInitialize(initConfig)

	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.xssh.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	if len(os.Args) >= 2 && os.Args[1] != "completion" {
		os.Stderr.WriteString(banner)
		rootCmd.PersistentFlags().StringVarP(&keyFile, "key-file", "i", keyFile, "ssh id file")
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".xssh" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".xssh")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

const banner = `   _  __     __________ __  __
  | |/ /    / ___/ ___// / / /
  |   /_____\__ \\__ \/ /_/ / 
 /   /_____/__/ /__/ / __  /  
/_/|_|    /____/____/_/ /_/   
                              
   The Extreme SSH Tool

Home Page: https://github.com/moisespsena-go/xssh
   Author: Moises P. Sena


`
