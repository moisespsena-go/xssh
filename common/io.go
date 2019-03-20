package common

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
)

type Copier struct {
	name string
	w    io.Writer
	r    io.Reader
}

func NewCopier(name string, w io.Writer, r io.Reader) *Copier {
	return &Copier{name: name, w: w, r: r}
}

func (cp Copier) String() string {
	return cp.name
}

func (cp Copier) Copy() error {
	_, err := io.Copy(cp.w, cp.r)
	if err != nil && err != io.EOF {
		log.Println("io.Copy ["+cp.name+"] error:", err)
		return err
	}
	return nil
}

func RemoveEmptyDir(pth string, count int) (err error) {
	for i := 0; i < count; i++ {
		var f *os.File
		if f, err = os.Open(pth); err != nil {
			return errors.New("open dir `" + pth + "` failed: " + err.Error())
		}
		var list []string
		list, err = f.Readdirnames(1)
		f.Close()
		if err != nil {
			if err == io.EOF {
				err = nil
			} else {
				return errors.New("readdirnames `" + pth + "` failed: " + err.Error())
			}
		}
		if len(list) != 0 {
			return
		}
		pth = filepath.Dir(pth)
	}
	return
}

func Digest(pth string) (v string, err error) {
	f, err := os.Open(pth)
	if err != nil {
		err = fmt.Errorf("open exe failed: %v", err)
		return
	}
	defer f.Close()

	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		err = fmt.Errorf("hash process failed: %v", err)
		return
	}

	d := digest.NewDigest(digest.SHA512, hash)

	v = d.String()
	return
}
