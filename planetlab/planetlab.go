package planetlab

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	Run        = 1
	Start      = 2
	ClientPath = "cmd/firechainClient/firechainClient"
	CaPath     = "cmd/ca/ca"
	AddrPath   = "planetlab/node_addrs"
	ClientCmd  = "./firechainClient"
	CaCmd      = "./ca"
	CleanCmd   = "pkill -9 firechainClient"
	CaCleanCmd = "pkill -9 ca"
	AliveCmd   = "ps aux | grep -c firechainClient"
)

var (
	errNoKeys = errors.New("No hostkeys found for hosts")
)

type CmdStatus struct {
	Success bool
	Addr    string
	Result  string
}

func decrypt(key []byte, password []byte) []byte {
	block, rest := pem.Decode(key)
	if len(rest) > 0 {
		log.Fatalf("Extra data included in key")
	}
	der, err := x509.DecryptPEMBlock(block, password)
	if err != nil {
		log.Fatalf("Decrypt failed: %v", err)
	}
	return der
}

func GenSshConfig() (*ssh.ClientConfig, error) {
	//var hostKey ssh.PublicKey
	// A public key may be used to authenticate against the remote
	// server by using an unencrypted PEM-encoded private key file.
	//
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.
	/*
		hostKeys, err := getHostKey(addr)
		if err != nil {
			return nil, err
		}
	*/

	b, err := ioutil.ReadFile("/home/jon/.ssh/id_rsa")
	if err != nil {
		return nil, err
	}

	der := decrypt(b, []byte("feeder123"))

	key, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, err
	}

	// Create the Signer for this private key.
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User: "uitple_firechain",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		//Config: ssh.Config{
		//	KeyExchanges: hostKeys, //[]string{hostKey.Type()},
		//},
		HostKeyCallback: check,
		Timeout:         time.Second * 20,
	}, nil
}

func check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

func TransferToHosts(addrs []string, path string, conf *ssh.ClientConfig, c chan *CmdStatus) {
	for _, a := range addrs {
		go transfer(a, path, conf, c)
	}
}

func DoCmds(addrs []string, mode int, cmd string, conf *ssh.ClientConfig, c chan *CmdStatus) {
	for _, a := range addrs {
		go doCmd(a, mode, cmd, conf, c)
	}
}

func doCmd(addr string, mode int, cmd string, conf *ssh.ClientConfig, ch chan *CmdStatus) {
	success := true
	out, err := ExecuteCmd(addr, cmd, conf, mode)
	if err != nil {
		fmt.Println(err)
		success = false
	}

	status := &CmdStatus{
		Addr:    addr,
		Success: success,
		Result:  out,
	}

	ch <- status
}

func transfer(addr string, path string, conf *ssh.ClientConfig, ch chan *CmdStatus) {
	success := true
	err := transferFile(addr, path, conf)
	if err != nil {
		log.Println(err)
		success = false
	}

	status := &CmdStatus{
		Addr:    addr,
		Success: success,
	}
	ch <- status
}

func ExecuteCmd(addr string, cmd string, conf *ssh.ClientConfig, mode int) (string, error) {
	client, err := ssh.Dial("tcp", addr+":22", conf)
	if err != nil {
		return "", err
	}

	// Each ClientConn can support multiple interactive sessions,
	// represented by a Session.
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return "", err
	}
	defer session.Close()

	// Once a Session is created, you can execute a single command on
	// the remote side using the Run method.
	var b bytes.Buffer
	session.Stdout = &b

	if mode == Run {
		if err := session.Run(cmd); err != nil {
			return "", err
		}
	} else if mode == Start {
		if err := session.Start(cmd); err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func transferFile(addr string, fp string, conf *ssh.ClientConfig) error {
	conn, err := ssh.Dial("tcp", addr+":22", conf)
	if err != nil {
		return err
	}

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	file, err := os.Open(fp)
	if err != nil {
		return err
	}
	defer file.Close()

	contents_bytes, _ := ioutil.ReadAll(file)
	r := bytes.NewReader(contents_bytes)
	size := len(contents_bytes)
	fileName := filepath.Base(file.Name())

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fmt.Fprintln(w, "C"+"0743", size, fileName)
		io.Copy(w, r)
		fmt.Fprintln(w, "\x00")
	}()

	session.Run("/usr/bin/scp -t /home/uitple_firechain/")

	return nil
}

func getHostKey(host string) ([]string, error) {
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var keyTypes []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		_, hosts, hostKey, _, _, err := ssh.ParseKnownHosts(scanner.Bytes())
		if err != nil {
			fmt.Println(err)
			continue
		}

		if hosts[1] == host {
			keyTypes = append(keyTypes, hostKey.Type())
			/*
				fmt.Println("Found hostKey for :", host)
				return hostKey, nil
			*/
		}
	}
	if len(keyTypes) == 0 {
		return nil, errNoKeys
	}
	return keyTypes, nil
}
