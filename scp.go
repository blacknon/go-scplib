package scplib

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
)

type SCPClient struct {
	Addr    string
	Port    string
	User    string
	Pass    string
	KeyPath string
	Connect *ssh.Client
}

func unset(s []string, i int) []string {
	if i >= len(s) {
		return s
	}
	return append(s[:i], s[i+1:]...)
}

func getFullPath(path string) (fullPath string) {
	usr, _ := user.Current()
	fullPath = strings.Replace(path, "~", usr.HomeDir, 1)
	fullPath, _ = filepath.Abs(fullPath)
	return fullPath
}

func walkDir(dir string) (files []string, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			path = path + "/"
		}
		files = append(files, path)
		return nil
	})
	return
}

func dirData(baseDir string, path string, toName string) (data string) {
	buf := []string{}

	// baseDirだと親ディレクトリ配下のみを転送するため、一度配列化して親ディレクトリも転送対象にする
	baseDirSlice := strings.Split(baseDir, "/")
	baseDirSlice = unset(baseDirSlice, len(baseDirSlice)-1)
	baseDir = strings.Join(baseDirSlice, "/")

	relPath, _ := filepath.Rel(baseDir, path)
	dir := filepath.Dir(relPath)

	if len(dir) > 0 && dir != "." {
		dirList := strings.Split(dir, "/")
		dirpath := baseDir
		for _, dirName := range dirList {
			dirpath = dirpath + "/" + dirName
			dInfo, _ := os.Stat(dirpath)
			dPerm := fmt.Sprintf("%04o", dInfo.Mode().Perm())
			buf = append(buf, fmt.Sprintln("D"+dPerm, 0, dirName))
		}
	}

	fInfo, _ := os.Stat(path)

	if !fInfo.IsDir() {
		content, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		fPerm := fmt.Sprintf("%04o", fInfo.Mode())
		buf = append(buf, fmt.Sprintln("C"+fPerm, len(content), toName))
		buf = append(buf, fmt.Sprintf(string(content)))
		buf = append(buf, fmt.Sprintf("\x00"))
	}

	if len(dir) > 0 && dir != "." {
		dirList := strings.Split(dir, "/")
		end_str := strings.Repeat("E\n", len(dirList))
		buf = append(buf, end_str)
	}
	data = strings.Join(buf, "")
	return
}

//func writeData(strings) {
//
//}

//func (s *SCPClient) CreateConnect() (conn *ssh.Client, err error) {
func (s *SCPClient) CreateConnect() (err error) {
	usr, _ := user.Current()
	auth := []ssh.AuthMethod{}
	if s.KeyPath != "" {
		s.KeyPath = strings.Replace(s.KeyPath, "~", usr.HomeDir, 1)
		// Read PublicKey
		buffer, b_err := ioutil.ReadFile(s.KeyPath)
		if b_err != nil {
			err = b_err
			return
		}
		key, b_err := ssh.ParsePrivateKey(buffer)
		if b_err != nil {
			err = b_err
			return
		}
		auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else {
		auth = []ssh.AuthMethod{ssh.Password(s.Pass)}
	}

	config := &ssh.ClientConfig{
		User:            s.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}

	// New connect
	conn, err := ssh.Dial("tcp", s.Addr+":"+s.Port, config)
	s.Connect = conn
	return
}

// Remote to Local get file
func (s *SCPClient) GetFile(fromPath string, toPath string) (err error) {
	defer s.Connect.Close()

	// New Session
	session, err := s.Connect.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error %v,cannot open new session: %v \n", err)
	}
	defer session.Close()

	fin := make(chan bool)
	go func() {
		w, _ := session.StdinPipe()

		defer w.Close()

		// Null Characters(10,000)
		nc := strings.Repeat("\x00", 10000)
		fmt.Fprintf(w, nc)
	}()

	go func() {
		r, _ := session.StdoutPipe()
		b, _ := ioutil.ReadAll(r)
		// 処理データをここで渡す？メモリに書き出すのは大きいファイル処理時にメモリを食い過ぎるので、パイプから直接書き出したい
		// また、処理時に名称切り替えに対応する必要があるので、それも考慮する
		fmt.Println(string(b))
		fin <- true
	}()

	if err := session.Run("/usr/bin/scp -rqf " + fromPath); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
	}

	<-fin
	return
}

// Local to Remote put file
func (s *SCPClient) PutFile(fromPath string, toPath string) (err error) {
	defer s.Connect.Close()

	// Get full path
	fromPath = getFullPath(fromPath)

	// New Session
	session, err := s.Connect.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	// File or Dir exits check
	pInfo, err := os.Stat(fromPath)
	if err != nil {
		return
	}

	// Read Dir or File
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		if pInfo.IsDir() {
			pList, _ := conf.PathWalkDir(fromPath)
			for _, i := range pList {
				data := dirData(fromPath, i, filepath.Base(i))
				if len(data) > 0 {
					fmt.Fprintf(w, data)
				}
			}
		} else {
			toPath = filepath.Base(toPath)

			// get file permission
			fPerm := fmt.Sprintf("%04o", pInfo.Mode())

			// get contents data
			content, err := ioutil.ReadFile(fromPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}

			// print scp format data
			fmt.Fprintln(w, "C"+fPerm, len(content), toPath)
			fmt.Fprint(w, string(content))
			fmt.Fprint(w, "\x00")
		}
	}()

	if err := session.Run("/usr/bin/scp -ptr " + toPath); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
	}
	return
}

//func (s *SCPClient) GetRemoteDataString(Path string) (getData string, err error) {
//
//}

// Return local data return scp format
func (s *SCPClient) GetLocalDataString(fromPath string, toPath string) (getData string, err error) {
	// Get full path
	fromPath = getFullPath(fromPath)

	// File or Dir exits check
	pInfo, err := os.Stat(fromPath)
	if err != nil {
		return
	}

	w := bytes.Buffer{}
	// Read Dir or File
	if pInfo.IsDir() {
		pList, _ := conf.PathWalkDir(fromPath)
		for _, i := range pList {
			data := dirData(fromPath, i, filepath.Base(i))
			if len(data) > 0 {
				w.WriteString(data)
			}
		}
	} else {
		fPerm := "0644"
		toPath = filepath.Base(toPath)
		content, err := ioutil.ReadFile(fromPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		w.WriteString("C" + fPerm + " " + strconv.Itoa(len(content)) + " " + toPath + "\n")
		w.WriteString(string(content))
		w.WriteString("\x00")
	}

	// nil charが出てきてしまうので、対応が必要
	getData = w.String()
	return
}

//func (s *SCPClient) PutData(fromData string, toPath string) (err error) {
//
//}
