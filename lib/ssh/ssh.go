// Package ssh provides high-level wrapper of golang.org/x/crypto/ssh
package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/golang/glog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

const (
	// wellKnownPort is the well-known port of SSH
	wellKnownPort = 22
)

type SSH struct {
	cfg ssh.ClientConfig
}

func WithPrivateKeyFile(user, fname string) (SSH, error) {
	p, err := ioutil.ReadFile(fname)
	if err != nil {
		return SSH{}, err
	}
	s, err := ssh.ParsePrivateKey(p)
	if err != nil {
		return SSH{}, err
	}
	return SSH{
		cfg: ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{ssh.PublicKeys(s)},
		},
	}, nil
}

// Output runs the given command on the remote server.
// It returns the stdout outputs of the command.
func (s SSH) Output(ctx context.Context, host, cmd string) ([]byte, error) {
	// TODO(yugui) Support IPv6 address without port number
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, fmt.Sprintf("%d", wellKnownPort))
	}
	glog.V(1).Infof("Running %q in %s@%s", cmd, s.cfg.User, host)
	client, err := ssh.Dial("tcp", host, &s.cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf

	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			if err := session.Signal(ssh.SIGHUP); err != nil {
				glog.Errorf("Failed to send SIGHUP to the remote session (%s@%s)", s.cfg.User, host)
			}
		}
	}()
	err = func() error {
		defer close(done)
		return session.Run(cmd)
	}()
	if err != nil {
		return nil, fmt.Errorf("cannot run cmd %q on host %s: %v: %s", cmd, host, err, errBuf.String())
	}
	return outBuf.Bytes(), nil
}
