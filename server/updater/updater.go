package updater

import (
	"io"
	"log"

	"github.com/gliderlabs/ssh"
	"github.com/moisespsena-go/xssh/common"
)

type Updater interface {
	Execute(uc *UpdaterClient, payload common.ApUpgradePayload) (err error)
}

type UpdaterClient struct {
	ssh.Session
	Name string
}

func NewUpdaterClient(session ssh.Session, name string) *UpdaterClient {
	return &UpdaterClient{Session: session, Name: name}
}

func (uc *UpdaterClient) Log(args ...interface{}) {
	args = append([]interface{}{"[" + uc.Name + "]"}, args...)
	log.Println(args...)
}

func (uc *UpdaterClient) Err(msg string) {
	var (
		up  common.UpgradePayload
		err error
	)
	if err = up.ErrorF(uc, msg); err != nil {
		if err == io.EOF {
			return
		}
		uc.Log("write upgrade payload failed: " + msg)
	} else {
		uc.Log(msg)
	}
}

func (uc *UpdaterClient) Sync(payload common.ApUpgradePayload, w io.Writer, r io.Reader) (err error) {
	go common.NewCopier(uc.Name+" <- upgrader", uc, r).Copy()
	if err = payload.FPrint(w); err != nil {
		return
	}
	return common.NewCopier(uc.Name+" -> upgrader", w, uc).Copy()
}
