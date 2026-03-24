package sshops

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func dialSSH(host *HostConfig, timeoutSec int) (*ssh.Client, error) {
	authMethods, err := buildAuthMethods(host)
	if err != nil {
		return nil, err
	}
	if len(authMethods) == 0 {
		return nil, NewUserError("auth_invalid", "no SSH auth methods are configured", fmt.Errorf("host %q", host.ID))
	}

	hostKeyCallback, err := buildHostKeyCallback(host)
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User:            host.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Duration(timeoutSec) * time.Second,
	}

	address := fmt.Sprintf("%s:%d", host.Address, host.Port)
	client, err := ssh.Dial("tcp", address, cfg)
	if err != nil {
		return nil, NewUserError("ssh_connect_failed", "failed to connect over SSH", err)
	}
	return client, nil
}

func buildAuthMethods(host *HostConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if host.Auth.PrivateKey != "" || host.Auth.PrivateKeyPath != "" {
		signer, err := loadPrivateKey(host)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if host.Auth.Password != "" {
		methods = append(methods, ssh.Password(host.Auth.Password))
	}

	return methods, nil
}

func loadPrivateKey(host *HostConfig) (ssh.Signer, error) {
	var keyBytes []byte
	switch {
	case host.Auth.PrivateKey != "":
		keyBytes = []byte(host.Auth.PrivateKey)
	case host.Auth.PrivateKeyPath != "":
		raw, err := os.ReadFile(host.Auth.PrivateKeyPath)
		if err != nil {
			return nil, NewUserError("auth_invalid", "failed to read private key", err)
		}
		keyBytes = raw
	default:
		return nil, NewUserError("auth_invalid", "private key is not configured", nil)
	}

	if host.Auth.Passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(host.Auth.Passphrase))
		if err != nil {
			return nil, NewUserError("auth_invalid", "failed to parse encrypted private key", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, NewUserError("auth_invalid", "failed to parse private key", err)
	}
	return signer, nil
}

func buildHostKeyCallback(host *HostConfig) (ssh.HostKeyCallback, error) {
	switch host.HostKey.Mode {
	case "insecure_ignore":
		return ssh.InsecureIgnoreHostKey(), nil
	case "known_hosts":
		callback, err := knownhosts.New(host.HostKey.KnownHostsPath)
		if err != nil {
			return nil, NewUserError("host_key_invalid", "failed to load known_hosts file", err)
		}
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return callback(hostname, remote, key)
		}, nil
	default:
		return nil, NewUserError("host_key_invalid", "unsupported host key mode", fmt.Errorf("%q", host.HostKey.Mode))
	}
}
