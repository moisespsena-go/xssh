package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type Version struct {
	Version   string    `yaml:"Version"`
	Commit    string    `yaml:"Commit"`
	BuildDate time.Time `yaml:"Build Date"`
	OS        string    `yaml:"OS"`
	Arch      string    `yaml:"Arch"`
	Arm       uint8     `yaml:"Arm"`
	Digest    string    `yaml:"Digest"`
}

func (v *Version) ToString() string {
	var r = make([]string, 7)
	r[0] = v.Version
	r[1] = v.Commit
	if !v.BuildDate.IsZero() {
		r[2] = v.BuildDate.Format(time.RFC3339)
	}
	r[3] = v.OS
	r[4] = v.Arch
	r[5] = fmt.Sprint(v.Arm)
	r[6] = v.Digest
	return strings.Join(r, ":")
}

func (v *Version) Marshal() []byte {
	return []byte(v.ToString())
}

func (v *Version) FPrint(w io.Writer) (err error) {
	s := v.ToString()
	if err = binary.Write(w, binary.BigEndian, uint16(len(s))); err != nil {
		if err != io.EOF {
			err = fmt.Errorf("write version length failed: %v", err)
		}
		return
	}
	_, err = w.Write([]byte(s))
	return
}

func (v *Version) Unmarshal(b []byte) *Version {
	if len(b) == 0 {
		return v
	}
	s := string(b)
	parts := strings.SplitN(s, ":", 7)
	v.Version, v.Commit, v.OS, v.Arch, v.Digest = parts[0], parts[1], parts[3], parts[4], parts[6]
	if parts[2] != "" {
		v.BuildDate, _ = time.Parse(time.RFC3339, parts[2])
	}
	i, _ := strconv.Atoi(parts[5])
	v.Arm = uint8(i)

	return v
}

func (v *Version) FRead(r io.Reader) (err error) {
	var l uint16
	if err = binary.Read(r, binary.BigEndian, &l); err != nil {
		if err != io.EOF {
			err = fmt.Errorf("read version length failed: %v", err)
		}
		return
	}
	if l == 0 {
		return
	}
	var b = make([]byte, l)
	if _, err = r.Read(b); err != nil {
		if err != io.EOF {
			err = fmt.Errorf("read version content failed: %v", err)
		}
		return
	}
	v.Unmarshal(b)
	return nil
}
