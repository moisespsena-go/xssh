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

package main

import (
	"runtime"
	"time"
	_ "unsafe"

	"github.com/moisespsena-go/xssh/cmd"
	"github.com/moisespsena-go/xssh/common"
)

var (
	version, commit, date string
)

//go:linkname goarm runtime.goarm
var goarm uint8

func main() {
	cmd.Version = common.Version{
		Version: version,
		Commit:  commit,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Arm:     goarm,
	}
	if date != "" {
		cmd.Version.BuildDate, _ = time.Parse(time.RFC3339, date)
	}
	cmd.Execute()
}
