package server

import (
	"golang.org/x/crypto/ssh"
	"io"
)

type PubKeySigner struct {
	key ssh.PublicKey
}

func (s PubKeySigner) PublicKey() ssh.PublicKey {
	return s.key
}

func (s PubKeySigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	return nil, nil
}

