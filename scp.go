package scplib

import "golang.org/x/crypto/ssh"

type SCPClinet struct {
	Connect *ssh.Client
}

func (r *RunInfoScp) GetFile(fromPath string, toPath string) (err error) {

}

func (r *RunInfoScp) PutFile(fromPath string, toPath string) (err error) {

}

func (r *RunInfoScp) GetData(fromPath) (getData string, err error) {

}

func (r *RunInfoScp) PutData(fromData string, toPath string) (err error) {

}
