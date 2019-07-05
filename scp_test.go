package scplib_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/blacknon/go-scplib"
	"golang.org/x/crypto/ssh"
)

func Example() {
	// Read Private key
	key, err := ioutil.ReadFile(os.Getenv("HOME") + "/.ssh/id_rsa")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read private key: %v\n", err)
		os.Exit(1)
	}

	// Parse Private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse private key: %v\n", err)
		os.Exit(1)
	}

	// Create ssh client config
	config := &ssh.ClientConfig{
		User: "user",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}

	// Create ssh connection
	connection, err := ssh.Dial("tcp", "test-node:22", config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to dial: %s\n", err)
		os.Exit(1)
	}
	defer connection.Close()

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false // copy permission with scp flag
	scp.Connection = connection

	// scp get file
	// scp.GetFile("/From/Remote/Path","/To/Local/Path")
	err = scp.GetFile([]string{"/etc/passwd"}, "./passwd")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp get: %s\n", err)
		os.Exit(1)
	}

	// // Dir pattern snip // this sample comment out
	// err := scp.GetFile("/path/from/remote/dir", "./path/to/local/dir")
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Failed to create session: %s\n", err)
	// 	os.Exit(1)
	// }

	// scp put file
	// scp.PutFile("/From/Local/Path","/To/Remote/Path")
	err = scp.PutFile([]string{"./passwd"}, "./passwd_scp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}

	// scp get file (to scp format data)
	// scp.GetData("/path/remote/path")
	getData, err := scp.GetData([]string{"/etc/passwd"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}

	fmt.Println(getData)

	// scp put file (to scp format data)
	// scp.GetData(Data,"/path/remote/path")
	err = scp.PutData(getData, "./passwd_data")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}
}

func ExampleSCPClient_GetFile() {
	var connection *ssh.Client

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false      // copy permission with scp flag
	scp.Connection = connection // *ssh.Client

	// Get /etc/passwd from remote machine, and copy to ./passwd
	err := scp.GetFile([]string{"/etc/passwd"}, "./passwd")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp get: %s\n", err)
		os.Exit(1)
	}
}

func ExampleSCPClient_PutFile() {
	var connection *ssh.Client

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false      // copy permission with scp flag
	scp.Connection = connection // *ssh.Client

	// Put ./passwd to remote machine `./passwd_scp`
	err := scp.PutFile([]string{"./passwd"}, "./passwd_scp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}
}

func ExampleSCPClient_GetData() {
	var connection *ssh.Client

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false      // copy permission with scp flag
	scp.Connection = connection // *ssh.Client

	// Get /etc/passwd from remote machine
	getData, err := scp.GetData([]string{"/etc/passwd"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}

	// println getData
	fmt.Println(getData)

	// Output:
	// C0644 1561 passwd
	// root:x:0:0:root:/root:/bin/bash
	// daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
	// bin:x:2:2:bin:/bin:/usr/sbin/nologin
	// sys:x:3:3:sys:/dev:/usr/sbin/nologin
	// ...
}

func ExampleSCPClient_PutData() {
	var connection *ssh.Client

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false      // copy permission with scp flag
	scp.Connection = connection // *ssh.Client

	// Get /etc/passwd from remote machine
	getData, err := scp.GetData([]string{"/etc/passwd"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}

	// getData Value...
	// C0644 1561 passwd
	// root:x:0:0:root:/root:/bin/bash
	// daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
	// bin:x:2:2:bin:/bin:/usr/sbin/nologin
	// sys:x:3:3:sys:/dev:/usr/sbin/nologin
	// ...

	// Put getData
	err = scp.PutData(getData, "./passwd_data")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
		os.Exit(1)
	}
}

// Test GetFile in CricleCI
func TestCircleCIGetFile(t *testing.T) {
	// Read Private key
	key, err := ioutil.ReadFile("/.ssh/id_rsa")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read private key: %v\n", err)
		os.Exit(1)
	}

	// Parse Private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse private key: %v\n", err)
		os.Exit(1)
	}

	// Create ssh client config
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}

	// Create ssh connection
	connection, err := ssh.Dial("tcp", "test-node:50022", config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to dial: %s\n", err)
		os.Exit(1)
	}
	defer connection.Close()

	// Create scp client
	scp := new(scplib.SCPClient)
	scp.Permission = false // copy permission with scp flag
	scp.Connection = connection

	// scp get file
	// scp.GetFile("/From/Remote/Path","/To/Local/Path")
	err = scp.GetFile([]string{"/etc/passwd"}, "./passwd")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to scp get: %s\n", err)
		os.Exit(1)
	}
}
