// Package ssh provides high-level wrapper of golang.org/x/crypto/ssh
package ssh

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// RemoteCmdOutput runs the given command on a remote server at the given hostname as the given user.
// It returns the stdout outputs of the command.
func RemoteCmdOutput(username, hostname, cmd string, privateKey []byte) (b []byte, err error) {
	p, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return b, err
	}
	s, err := ssh.NewSignerFromKey(p)
	if err != nil {
		return b, err
	}
	pub := ssh.PublicKeys(s)
	clientConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{pub},
	}

	client, err := ssh.Dial("tcp", hostname, clientConfig)
	if err != nil {
		return b, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return b, err
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf
	if err := session.Run(cmd); err != nil {
		return b, fmt.Errorf("cannot run cmd %q on host %s: %v: %s", cmd, hostname, err, errBuf.String())
	}
	return outBuf.Bytes(), nil
}
