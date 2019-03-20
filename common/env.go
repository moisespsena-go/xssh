package common

import (
	"os"
	"strings"
)

func Environ() (env []string) {
	for _, e := range os.Environ() {
		if e == "GO_UPGRADE_BIN_CHECK" || strings.HasPrefix(e, "OVERSEER_") {
			continue
		}
		env = append(env, e)
	}
	return
}
