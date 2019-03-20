package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"gopkg.in/yaml.v2"

	"github.com/go-errors/errors"
)

type UpgradePayload struct {
	Ok         bool
	Changed    bool
	Stream     bool
	StreamSize int64
	URL        string
	Err        error
}

func (up *UpgradePayload) Read(in io.Reader) (r io.Reader, err error) {
	if err = binary.Read(in, binary.BigEndian, &up.Ok); err != nil {
		if err == io.EOF {
			return
		}
		return nil, fmt.Errorf("read Ok field failed: %v", err)
	} else if up.Ok {
		if err = binary.Read(in, binary.BigEndian, &up.Changed); err != nil {
			if err == io.EOF {
				return
			}
			return nil, fmt.Errorf("read Changed field failed: %v", err)
		} else if up.Changed {
			if err = binary.Read(in, binary.BigEndian, &up.Stream); err != nil {
				if err == io.EOF {
					return
				}
				return nil, fmt.Errorf("read Stream field failed: %v", err)
			} else if up.Stream {
				if err = binary.Read(in, binary.BigEndian, &up.StreamSize); err != nil {
					if err == io.EOF {
						return
					}
					return nil, fmt.Errorf("read StreamSize field failed: %v", err)
				}
				r = io.LimitReader(in, up.StreamSize)
			} else {
				var l uint16
				if err = binary.Read(in, binary.BigEndian, &l); err != nil {
					if err == io.EOF {
						return
					}
					return nil, fmt.Errorf("read URL len failed: %v", err)
				}
				b := make([]byte, int(l))
				var n int
				if n, err = in.Read(b); err != nil {
					if err == io.EOF {
						return
					}
					return nil, fmt.Errorf("read URL failed: %v", err)
				}
				if n != int(l) {
					return nil, fmt.Errorf("bad URL len: expected %d but get %d", int(l), n)
				}
				up.URL = string(b)
			}
		}
	} else if err = binary.Read(in, binary.BigEndian, &up.StreamSize); err != nil {
		if err == io.EOF {
			return
		}
		return nil, fmt.Errorf("read StreamSize field failed: %v", err)
	} else if up.StreamSize == 0 {
		up.Err = errors.New("<empty message>")
	} else {
		var b = make([]byte, up.StreamSize)
		if _, err = in.Read(b); err != nil {
			return nil, fmt.Errorf("read Error message failed: %v", err)
		}
		up.Err = errors.New(string(b))
	}

	return
}

func (up *UpgradePayload) Write(w io.Writer) (err error) {
	if err = binary.Write(w, binary.BigEndian, true); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("write ok field failed: %v", err)
	} else if err = binary.Write(w, binary.BigEndian, up.Changed); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("write changed field failed: %v", err)
	} else if up.Changed {
		if err = binary.Write(w, binary.BigEndian, up.Stream); err != nil {
			if err == io.EOF {
				return err
			}
			return fmt.Errorf("write stream field failed: %v", err)
		} else if up.Stream {
			if err = binary.Write(w, binary.BigEndian, up.StreamSize); err != nil {
				if err == io.EOF {
					return err
				}
				return fmt.Errorf("write StreamSize field failed: %v", err)
			}
		} else {
			if err = binary.Write(w, binary.BigEndian, uint16(len(up.URL))); err != nil {
				if err == io.EOF {
					return err
				}
				return fmt.Errorf("write URL len failed: %v", err)
			}
			_, err = w.Write([]byte(up.URL))
		}
	}
	return
}

func (up *UpgradePayload) ErrorF(w io.Writer, msg interface{}) (rerr error) {
	var err error
	switch msgt := msg.(type) {
	case error:
		err = msgt
	case string:
		err = errors.New(msgt)
	default:
		err = errors.New(fmt.Sprint(msgt))
	}
	if rerr = binary.Write(w, binary.BigEndian, false); rerr != nil {
		if rerr == io.EOF {
			return rerr
		}
		return fmt.Errorf("write ok field failed: %v", rerr)
	}

	var errSize int
	if err != nil {
		errSize = len(err.Error())
	}

	if rerr = binary.Write(w, binary.BigEndian, uint16(errSize)); rerr != nil {
		if rerr == io.EOF {
			return rerr
		}
		return fmt.Errorf("write err size failed: %v", rerr)
	}
	_, rerr = w.Write([]byte(err.Error()))
	return
}

type ApUpgradePayload struct {
	Ap      string  `yaml:"Ap"`
	ApAddr  string  `yaml:"ApAddr"`
	Version Version `yaml:"Version"`
}

func (up ApUpgradePayload) String() string {
	var out bytes.Buffer
	up.FPrint(&out)
	return out.String()
}

func (up *ApUpgradePayload) FPrint(w io.Writer) (err error) {
	if err = yaml.NewEncoder(w).Encode(up); err != nil {
		return fmt.Errorf("marshall version failed: %v", err)
	}
	return nil
}

func (up *ApUpgradePayload) FRead(r io.Reader) (err error) {
	if err = yaml.NewDecoder(r).Decode(up); err != nil {
		return fmt.Errorf("unmarshall ap upgrade payload failed: %v", err)
	}
	return nil
}
