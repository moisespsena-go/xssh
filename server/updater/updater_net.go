package updater

import (
	"fmt"
	"net"
	"strings"

	"github.com/moisespsena-go/xssh/common"
)

type NetUpdater struct {
	Addr string
}

func NewNetUpdater(addr string) *NetUpdater {
	return &NetUpdater{Addr: addr}
}

func (u NetUpdater) Execute(uc *UpdaterClient, payload common.ApUpgradePayload) (err error) {
	var (
		con net.Conn
	)

	if strings.HasPrefix(u.Addr, "unix:") {
		con, err = net.Dial("unix", strings.TrimPrefix(u.Addr, "unix:"))
	} else {
		con, err = net.Dial("tcp", u.Addr)
	}

	if err != nil {
		err = fmt.Errorf("proxy to updater server %s failed: %v", u.Addr, err)
		return
	}

	defer con.Close()

	uc.Sync(payload, con, con)
	return
}
