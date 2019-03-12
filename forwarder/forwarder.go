package forwarder

import (
	"io"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/moisespsena-go/task"

	"github.com/moisespsena-go/xssh/common"

	gossh "golang.org/x/crypto/ssh"
)

type Forwarder struct {
	ServerAddr string
	UserName   string
	ApName     string
	KeyFile    string
	services   []*Service

	client  *gossh.Client
	delayer *common.Delayer

	closed, stop, running bool
	sync.Mutex

	servicesStoper task.Stoper
}

func NewClient(userName string) *Forwarder {
	return &Forwarder{
		UserName: userName,
		delayer:  common.NewDelayer(time.Second * 10),
	}
}

func (fw *Forwarder) GetService(name string) (s *Service) {
	for _, s = range fw.services {
		if s.Name == name {
			return
		}
	}
	return nil
}

func (fw *Forwarder) Close() error {
	fw.closed = true
	if fw.client != nil {
		return fw.client.Close()
	}
	return fw.delayer.Close()
}

func (fw *Forwarder) AddServices(s ...*Service) {
	fw.services = append(fw.services, s...)
	for _, s := range s {
		s.fw = fw
	}
}

func (fw *Forwarder) run() {
	fw.closed = false
	var err error
	fw.client, err = fw.connectToHost()
	if err != nil {
		log.Println("Connect to server", fw.ServerAddr, "failed:", err)
		return
	}
	log.Println("Connected to server", fw.ServerAddr)

	var (
		tasks task.Slice
	)

	for _, s := range fw.services {
		s.stop = false
		tasks = append(tasks, s)
	}

	fw.Lock()
	defer fw.Unlock()

	fw.servicesStoper, err = task.Start(nil, tasks...)
	if err != nil {
		log.Println("Start services failed:", err)
		return
	}

	go func() {
		if err := fw.client.Wait(); err != nil && err != io.EOF {
			log.Println("SSH client closed with error: ", err)
		} else {
			log.Println("SSH client closed")
		}

		fw.Lock()
		defer fw.Unlock()
		fw.client = nil

		if fw.servicesStoper != nil {
			fw.servicesStoper.Stop()
			go func() {
				for fw.servicesStoper.IsRunning() {
					log.Println("Wait to stop services")
					<-time.After(time.Second * 5)
				}
			}()
		}
	}()
}

func (fw *Forwarder) SetReconnectTimeout(t time.Duration) {
	fw.delayer.SetDuration(t)
}

func (fw Forwarder) connectToHost() (*gossh.Client, error) {
	buf, err := ioutil.ReadFile(common.GetKeyFile(fw.KeyFile))
	if err != nil {
		log.Fatalf("Load Key filed %v", err)
	}
	key, err := gossh.ParsePrivateKey(buf)
	if err != nil {
		log.Fatalf("Parse Key filed %v", err)
	}

	sshConfig := &gossh.ClientConfig{
		User: fw.UserName + ":" + fw.ApName,
		Auth: []gossh.AuthMethod{gossh.PublicKeys(key)},
	}
	sshConfig.HostKeyCallback = gossh.InsecureIgnoreHostKey()

	client, err := gossh.Dial("tcp", fw.ServerAddr, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (fw *Forwarder) forever(done ...func()) {
	defer func() {
		for _, done := range done {
			done()
		}
	}()
	for !fw.closed && !fw.stop {
		if fw.client == nil {
			fw.run()
		}
		fw.delayer.Wait()
	}
}

func (fw *Forwarder) setup() {
	if fw.ServerAddr == "" {
		fw.ServerAddr = common.DefaultServerAddr
	}
}

func (fw *Forwarder) Stop() {
	fw.Lock()
	defer fw.Unlock()
	fw.stop = true
	if fw.servicesStoper != nil {
		fw.servicesStoper.Stop()
	}
	fw.delayer.Close()
}

func (fw *Forwarder) IsRunning() bool {
	fw.Lock()
	defer fw.Unlock()
	return fw.running || fw.servicesStoper.IsRunning()
}

func (fw *Forwarder) Setup(appender task.Appender) (err error) {
	return nil
}

func (fw *Forwarder) Run() (err error) {
	fw.setup()
	fw.forever()
	return nil
}

func (fw *Forwarder) Start(done func()) (stop task.Stoper, err error) {
	fw.setup()
	go fw.forever(done)
	return fw, nil
}