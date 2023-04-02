// Package integration for integration tests
package integration

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid_proxy_server/pkg/types"
	"golang.org/x/crypto/ssh"
)

var (
	trueVal  = true
	statusUp = "up"
	value1   = uint64(1)
	value2   = uint64(2)
	value10  = uint64(10)
)

var nodeFilter = types.NodeFilter{
	Status:  &statusUp,
	FreeSRU: &value10,
	FreeHRU: &value2,
	FreeMRU: &value2,
	FarmIDs: []uint64{1},
	IPv6:    &trueVal,
}

func setup() (deployer.TFPluginClient, error) {
	mnemonics := os.Getenv("MNEMONICS")
	mnemonics = "winner giant reward damage expose pulse recipe manual brand volcano dry avoid"
	log.Printf("mnemonics: %s", mnemonics)

	network := "dev" // os.Getenv("NETWORK")
	log.Printf("network: %s", network)

	return deployer.NewTFPluginClient(mnemonics, "sr25519", network, "", "", "", 0, true, true)
}

// TestConnection used to test connection
func TestConnection(addr string, port string) bool {
	for t := time.Now(); time.Since(t) < 3*time.Second; {
		con, err := net.DialTimeout("tcp", net.JoinHostPort(addr, port), time.Second*12)
		if err == nil {
			con.Close()
			return true
		}
	}
	return false
}

// RemoteRun used for running cmd remotely using ssh
func RemoteRun(user string, addr string, cmd string, privateKey string) (string, error) {
	key, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", errors.Wrapf(err, "could not parse ssh private key %v", key)
	}
	// Authentication
	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	// Connect
	port := "22"
	client, err := ssh.Dial("tcp", net.JoinHostPort(addr, port), config)
	if err != nil {
		return "", errors.Wrapf(err, "could not start ssh connection")
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", errors.Wrapf(err, "could not create new session with message error")
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", errors.Wrapf(err, "could not execute command on remote with output %s", output)
	}
	return string(output), nil
}

// GenerateSSHKeyPair creates the public and private key for the machine
func GenerateSSHKeyPair() (string, string, error) {

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", "", errors.Wrapf(err, "could not generate rsa key")
	}

	pemKey := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}
	privateKey := pem.EncodeToMemory(pemKey)

	pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return "", "", errors.Wrapf(err, "could not extract public key")
	}
	authorizedKey := ssh.MarshalAuthorizedKey(pub)
	return string(authorizedKey), string(privateKey), nil
}
