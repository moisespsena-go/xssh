package common

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
)

func NewDelayer(duration time.Duration) *Delayer {
	return &Delayer{duration: duration}
}

type Delayer struct {
	sync.Mutex
	duration time.Duration
	done     chan interface{}
}

func (d *Delayer) Duration() time.Duration {
	return d.duration
}

func (d *Delayer) SetDuration(duration time.Duration) {
	if duration < time.Second {
		duration = time.Second
	}
	d.duration = duration
}

func (d Delayer) Close() error {
	d.Lock()
	defer d.Unlock()

	if d.done != nil {
		close(d.done)
	}
	return nil
}

func (d *Delayer) Wait() {
	d.Lock()
	d.done = make(chan interface{})
	d.Unlock()

	defer func() {
		d.Lock()
		d.done = nil
		d.Unlock()
	}()

	timer := time.After(d.duration)
	for {
		select {
		case <-timer:
			return
		case <-d.done:
			return
		default:
			<-time.After(time.Second * 2)
		}
	}
}

func GetKeyFile(path ...string) string {
	for _, p := range path {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	usr, err := user.Current()
	if err != nil {
		log.Fatalln("Get current user failed:", err)
	}

	p := filepath.Join(usr.HomeDir, ".ssh", "id_rsa")
	if _, err := os.Stat(p); err != nil {
		log.Fatalln(err)
	}
	return p
}
