package forwarder

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moisespsena-go/xssh/common"
)

type Creator struct {
	ServiceNames     []string
	ReconnectTimeout string
	DSN              string
	KeyFile          string
	Port             int
}

func (c Creator) Create() (client *Forwarder, err error) {
	if len(c.ServiceNames) == 0 {
		err = errors.New("No services")
		return
	}

	var d time.Duration
	if d, err = time.ParseDuration(c.ReconnectTimeout); err != nil {
		err = fmt.Errorf("bad reconnect-timeout value: %v", err)
		return
	}

	if d < time.Second {
		err = fmt.Errorf("bad reconnect-timeout value: minimum value is `1s` (one second)")
		return
	}

	var userName, serverAddr, apName string

	if parts := strings.Split(c.DSN, "@"); len(parts) == 2 {
		serverAddr = parts[1]
		parts := strings.Split(parts[0], ":")
		apName = parts[0]
		switch len(parts) {
		case 1:
			apName = parts[0]
			userName = common.CurrentUser.Username
		case 2:
			if parts[0] == "" {
				userName = common.CurrentUser.Username
			} else {
				userName = parts[0]
			}
			apName = parts[1]
		}
	}

	if apName == "" || userName == "" || serverAddr == "" {
		err = errors.New("bad DSN format: expected [USER:]AP_NAME@XSSH_SERVER_HOST")
		return
	}

	if c.Port == 0 {
		c.Port = 2220
	}

	client = NewClient(userName)
	client.ApName = apName
	client.ServerAddr = fmt.Sprintf("%v:%d", serverAddr, c.Port)
	client.KeyFile = c.KeyFile

	for i, name := range c.ServiceNames {
		parts := strings.SplitN(name, ":", 2)

		if len(parts) == 1 {
			parts = append(parts, "")
		}

		if parts[0] == "" {
			return nil, fmt.Errorf("service %d has empty name", i)
		}
		s, err := NewService(parts[0], parts[1])
		if err != nil {
			return nil, err
		}
		client.AddServices(s)
	}

	return
}
