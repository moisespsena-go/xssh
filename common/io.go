package common

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencontainers/go-digest"
)

type Copier struct {
	name    string
	w       io.Writer
	r       io.Reader
	closers []func() error
	closed  bool
	mu      sync.Mutex
}

func NewCopier(name string, w io.Writer, r io.Reader, closers ...func() error) *Copier {
	if len(closers) == 0 {
		if c, ok := w.(io.Closer); ok {
			closers = append(closers, c.Close)
		}
		if c, ok := r.(io.Closer); ok {
			closers = append(closers, c.Close)
		}
	}
	return &Copier{name: name, w: w, r: r, closers: closers}
}

func (cp *Copier) Close() error {
	if cp.closed {
		return nil
	}
	cp.mu.Lock()
	if cp.closed {
		return nil
	}
	cp.closed = true
	cp.mu.Unlock()

	for _, c := range cp.closers {
		c()
	}
	return nil
}

func (cp Copier) String() string {
	return cp.name
}

func (cp Copier) Copy() error {
	defer cp.Close()
	_, err := io.Copy(cp.w, cp.r)
	if err != nil {
		if err == io.EOF || strings.Contains(err.Error(), "closed network connection") {
			return io.EOF
		}
		log.Println("io.Copy ["+cp.name+"] error:", err)
		return err
	}
	return nil
}

func RemoveEmptyDir(rootDir, pth string) (err error) {
	if rootDir == "" || rootDir == "." {
		if rootDir, err = os.Getwd(); err != nil {
			return
		}
	} else if rootDir, err = filepath.Abs(rootDir); err != nil {
		return
	}

	if pth, err = filepath.Abs(pth); err != nil {
		return errors.New("abs path of `" + pth + "` from `" + rootDir + "` failed: " + err.Error())
	}

	if !strings.HasPrefix(pth, rootDir) {
		return errors.New("invalid sub path `" + pth + "`")
	}

	for pth != rootDir {
		var f *os.File
		if f, err = os.Open(pth); err != nil {
			return errors.New("open dir `" + pth + "` failed: " + err.Error())
		}
		var s os.FileInfo
		if s, err = f.Stat(); err != nil {
			f.Close()
			return errors.New("stat of `" + pth + "` failed: " + err.Error())
		}
		if !s.IsDir() {
			f.Close()
			return errors.New("`" + pth + "` not is dir")
		}
		_, err = f.Readdirnames(1)
		f.Close()
		if err != nil {
			if err == io.EOF {
				if err = os.Remove(pth); err != nil {
					return
				}
				pth = filepath.Dir(pth)
			} else {
				return errors.New("readdirnames `" + pth + "` failed: " + err.Error())
			}
		} else {
			return
		}
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

type IOSync struct {
	copiers []*Copier
	closed  bool
	mu      sync.Mutex
}

func NewIOSync(copiers ...*Copier) (s *IOSync) {
	s = &IOSync{copiers: copiers}
	for _, c := range copiers {
		c.closers = append(c.closers, s.close)
	}
	return s
}

func (s *IOSync) Sync() {
	defer s.close()
	for _, c := range s.copiers[1:] {
		go c.Copy()
	}
	s.copiers[0].Copy()
}

func (s *IOSync) close() error {
	if s.closed {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	for _, c := range s.copiers {
		c.Close()
	}
	return nil
}
