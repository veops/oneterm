package sshclient

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/veops/oneterm/pkg/config"
)

var hosts = struct {
	sync.Mutex
	path  string
	stamp time.Time
	size  int64
	check ssh.HostKeyCallback
}{}

func HostKey(address string) ssh.HostKeyCallback {
	return func(_ string, remote net.Addr, key ssh.PublicKey) error {
		path := config.Cfg.Ssh.KnownHosts
		if path == "" {
			path = filepath.Join(config.Cfg.Session.ReplayDir, "known_hosts")
		}
		hosts.Lock()
		defer hosts.Unlock()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			return err
		}
		if hosts.check == nil || hosts.path != path || !hosts.stamp.Equal(stat.ModTime()) || hosts.size != stat.Size() {
			hosts.check, err = knownhosts.New(path)
			if err != nil {
				return err
			}
			hosts.path, hosts.stamp, hosts.size = path, stat.ModTime(), stat.Size()
		}
		// Tunnels authenticate the original target, not their local forwarding port.
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		remote = &hostAddress{address: net.JoinHostPort(host, port)}
		if err = hosts.check(address, remote, key); err == nil {
			return nil
		}
		var mismatch *knownhosts.KeyError
		if !errors.As(err, &mismatch) || len(mismatch.Want) != 0 {
			return err
		}
		_, err = file.WriteString(knownhosts.Line([]string{knownhosts.Normalize(address)}, key) + "\n")
		hosts.check = nil
		return err
	}
}

type hostAddress struct{ address string }

func (a *hostAddress) Network() string { return "tcp" }
func (a *hostAddress) String() string  { return a.address }

func Dial(ctx context.Context, address string, configuration *ssh.ClientConfig) (*ssh.Client, error) {
	timeout := configuration.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	connection, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { connection.Close() })
	connection.SetDeadline(time.Now().Add(timeout))
	transport, channels, requests, err := ssh.NewClientConn(connection, address, configuration)
	if err != nil {
		stop()
		connection.Close()
		return nil, err
	}
	connection.SetDeadline(time.Time{})
	return ssh.NewClient(transport, channels, requests), nil
}
