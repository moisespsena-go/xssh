package ap

import (
	"io"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/moisespsena-go/xssh/common"

	gossh "golang.org/x/crypto/ssh"
)

type Ap struct {
	ID         string
	ServerAddr string
	ApName     string
	KeyFile    string
	Version    *common.Version
	Services   map[string]*Service

	client *gossh.Client
	closed bool
	sync.Mutex
	reconnectingMux sync.Mutex

	wg *sync.WaitGroup

	delayer         *common.Delayer
	registerDelayer *common.Delayer
	registered      map[string]*ServiceListener
}

func New(apName string) *Ap {
	c := &Ap{
		ApName:          apName,
		ServerAddr:      common.DefaultServerAddr,
		delayer:         common.NewDelayer(time.Second * 2),
		registerDelayer: common.NewDelayer(time.Second * 5),
		registered:      map[string]*ServiceListener{},
	}
	return c
}

func (c *Ap) SetReconnectTimeout(t time.Duration) {
	c.delayer.SetDuration(t)
}

func (c *Ap) Close() error {
	if c.closed {
		return nil
	}

	c.closed = true
	if c.client != nil {
		return c.client.Close()
	}

	for _, sl := range c.Services {
		sl.Close()
	}
	c.registerDelayer.Close()
	return c.delayer.Close()
}

func (c *Ap) run() {
	var err error
	c.client, err = c.connectToHost()

	log.Println("#"+c.ID+" connecting to server", c.ServerAddr)

	if err != nil {
		log.Println("#"+c.ID+" connect to server", c.ServerAddr, "failed:", err)
		return
	}
	log.Println("#"+c.ID+" connected to server", c.ServerAddr)

	if c.Version != nil {
		c.client.SendRequest("ap-version", false, []byte(c.Version.ToString()))
	}

	defer func() {
		c.registered = map[string]*ServiceListener{}
	}()

	go func() {
		if err := c.client.Wait(); err != nil && err != io.EOF {
			log.Println("#"+c.ID+" client closed with error: ", err)
		} else {
			log.Println("#" + c.ID + " client closed")
		}
		c.client = nil
	}()

	go func() {
		for c.client != nil && !c.closed {
			for name, sl := range c.Services {
				if _, ok := c.registered[name]; ok {
					continue
				}

				do := func(sl *Service) (*ServiceListener, bool) {
					log.Println("#" + c.ID + " {" + name + "} remote listen")
					ln, err := c.client.Listen("unix", sl.Name)
					if err != nil {
						log.Println("#"+c.ID+" {"+name+"} remote listen failed:", err)
						return nil, false
					}
					ssl := sl.Register(c.ID, ln)
					ssl.OnClose(func() {
						if c.registered != nil {
							if _, ok := c.registered[name]; ok {
								delete(c.registered, name)
							}
						}
					})
					return ssl, true
				}

				if ssl, ok := do(sl); ok {
					c.registered[name] = ssl
				} else {
					return
				}
			}
			c.registerDelayer.Wait()
		}
	}()

	defer func() {
		for _, ssl := range c.registered {
			ssl.Close()
		}
	}()

	for c.client != nil {
		<-time.After(time.Second * 30)
		if c.client != nil {
			if _, _, err := c.client.SendRequest("", false, nil); err != nil && c.client != nil {
				log.Println("#"+c.ID+" ERROR: failed to send PING request:", err.Error())
			}
		}
	}
}

func (c *Ap) remoteForever() {
	for !c.closed {
		c.run()
		if !c.closed {
			c.delayer.Wait()
		}
	}
}

func (c Ap) connectToHost() (*gossh.Client, error) {
	buf, err := ioutil.ReadFile(common.GetKeyFile(c.KeyFile))
	if err != nil {
		log.Fatalf("#"+c.ID+" Load Key failed %v", err)
	}
	key, err := gossh.ParsePrivateKey(buf)
	if err != nil {
		log.Fatalf("#"+c.ID+" parse Key failed %v", err)
	}

	sshConfig := &gossh.ClientConfig{
		User: c.ApName,
		Auth: []gossh.AuthMethod{gossh.PublicKeys(key)},
	}
	sshConfig.HostKeyCallback = gossh.InsecureIgnoreHostKey()

	client, err := gossh.Dial("tcp", c.ServerAddr, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Ap) Forever() {
	c.remoteForever()
}
