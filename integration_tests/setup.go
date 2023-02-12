// Package integration for integration tests
package integration

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/pkg/errors"
	"github.com/threefoldtech/grid3-go/deployer"
	"golang.org/x/crypto/ssh"
)

func setup() (deployer.TFPluginClient, error) {
	mnemonics := os.Getenv("MNEMONICS")
	log.Printf("mnemonics: %s", mnemonics)

	network := os.Getenv("NETWORK")
	log.Printf("network: %s", network)

	return deployer.NewTFPluginClient(mnemonics, "sr25519", network, "", "", true, true)
}

// TestConnection used to test connection
func TestConnection(addr string, port string) bool {
	for t := time.Now(); time.Since(t) < 3*time.Minute; {
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
		return "", "", errors.Wrapf(err, "Couldn't extract public key")
	}
	authorizedKey := ssh.MarshalAuthorizedKey(pub)
	return string(authorizedKey), string(privateKey), nil
}

// NodeFilter struct for options
type NodeFilter struct {
	CRU int `url:"free_cru,omitempty"` // GB
	MRU int `url:"free_mru,omitempty"` // GB
	SRU int `url:"free_sru,omitempty"` // GB
	HRU int `url:"free_hru,omitempty"` // GB

	PublicIPs bool `url:"ipv4,omitempty"`
	Gateway   bool `url:"domain,omitempty"`

	FarmID   string `url:"farm_ids,omitempty"`
	FarmName string `url:"farm_name,omitempty"`
	Country  string `url:"country,omitempty"`
	City     string `url:"city,omitempty"`

	Dedicated bool `url:"dedicated,omitempty"`
	Rentable  bool `url:"rentable,omitempty"`
	Rented    bool `url:"rented,omitempty"`

	AvailableForTwin int `url:"available_for,omitempty"`

	Page   int    `url:"page,omitempty"`
	Status string `url:"status,omitempty"`
}

// FilterNodes filters nodes on a network
func FilterNodes(options NodeFilter, url string) ([]uint32, error) {
	nodes := []uint32{}
	values, _ := query.Values(options)
	query := values.Encode()

	resp, err := http.Get(url + "/nodes?" + query)
	if err != nil {
		return nodes, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nodes, err
	}

	if body != nil {
		defer resp.Body.Close()
	}

	var nodesData []map[string]interface{}
	err = json.Unmarshal(body, &nodesData)
	if err != nil {
		return nodes, err
	}

	for _, node := range nodesData {
		nodes = append(nodes, uint32(node["nodeId"].(float64)))
	}

	if len(nodes) == 0 {
		return nodes, fmt.Errorf("couldn't find any node with options: %v", query)
	}

	return nodes, nil
}
