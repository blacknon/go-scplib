package scplib

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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

func readBytesAtSize(data *bufio.Reader, byteSize int) (readData []byte) {
	buff := make([]byte, byteSize)
	for i := 0; i < byteSize; i++ {
		readByte, err := data.ReadByte()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		buff = append(buff, readByte)
	}
	readData = buff
	return
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

func pushDirData(w io.WriteCloser, baseDir string, path string, toName string) {
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

			// push directory infomation
			fmt.Fprintln(w, "D"+dPerm, 0, dirName)
		}
	}

	fInfo, _ := os.Stat(path)

	if !fInfo.IsDir() {
		pushFileData(w, path, toName)
	}

	if len(dir) > 0 && dir != "." {
		dirList := strings.Split(dir, "/")
		end_str := strings.Repeat("E\n", len(dirList))
		fmt.Fprintf(w, end_str)
	}
	return
}

func pushFileData(w io.WriteCloser, path string, toName string) {
	content, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	stat, _ := content.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	fInfo, _ := os.Stat(path)
	fPerm := fmt.Sprintf("%04o", fInfo.Mode())

	// push file infomation
	fmt.Fprintln(w, "C"+fPerm, stat.Size(), toName)
	io.Copy(w, content)
	fmt.Fprint(w, "\x00")

	return
}

func writeData(data *bufio.Reader, path string) {
	pwd := path
checkloop:
	for {
		// Get file or dir infomation (1st line)
		line, err := data.ReadString('\n')

		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println(err)
		}

		line = strings.TrimRight(line, "\n")
		if line == "E" {
			pwd_array := strings.Split(pwd, "/")
			if len(pwd_array) > 0 {
				pwd_array = pwd_array[:len(pwd_array)-2]
			}
			pwd = strings.Join(pwd_array, "/") + "/"
			continue
		}

		line_slice := strings.SplitN(line, " ", 3)

		scpType := line_slice[0][:1]
		scpPerm := line_slice[0][1:]
		scpPerm32, _ := strconv.ParseUint(scpPerm, 8, 32)
		scpSize, _ := strconv.Atoi(line_slice[1])
		scpObjName := line_slice[2]

		switch {
		case scpType == "C":
			scpPath := path
			// Check pwd
			check, _ := regexp.MatchString("/$", pwd)
			if check || pwd != path {
				scpPath = pwd + scpObjName
			}

			fileData := readBytesAtSize(data, scpSize)
			ioutil.WriteFile(scpPath, fileData, os.FileMode(uint32(scpPerm32)))

			// read last nUll character
			_, _ = data.ReadByte()
		case scpType == "D":
			// Check pwd
			check, _ := regexp.MatchString("/$", pwd)
			if !check {
				pwd = pwd + "/"
			}

			pwd = pwd + scpObjName + "/"
			err := os.Mkdir(pwd, os.FileMode(uint32(scpPerm32)))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		default:
			fmt.Fprintln(os.Stderr, line)
			break checkloop
		}
	}
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
		nc := strings.Repeat("\x00", 100000)
		fmt.Fprintf(w, nc)
	}()

	go func() {
		r, _ := session.StdoutPipe()
		b := bufio.NewReader(r)
		writeData(b, toPath)

		fin <- true
	}()

	err = session.Run("/usr/bin/scp -rqf '" + fromPath + "'")
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
			// Directory
			pList, _ := walkDir(fromPath)
			for _, i := range pList {
				pushDirData(w, fromPath, i, filepath.Base(i))
			}
		} else {
			// single files
			toFile := filepath.Base(toPath)
			if toFile == "." {
				toFile = filepath.Base(fromPath)
			}
			pushFileData(w, fromPath, toFile)
		}
	}()

	err = session.Run("/usr/bin/scp -ptr '" + toPath + "'")

	return
}

//func (s *SCPClient) GetData(fromPath string) (err error) {
func (s *SCPClient) GetData(fromPath string) (data *bytes.Buffer, err error) {
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
		nc := strings.Repeat("\x00", 100000)
		fmt.Fprintf(w, nc)
	}()

	buf := new(bytes.Buffer)
	go func() {
		r, _ := session.StdoutPipe()
		buf.ReadFrom(r)
		fin <- true
	}()

	err = session.Run("/usr/bin/scp -rqf '" + fromPath + "'")
	<-fin
	data = buf
	return
}

func (s *SCPClient) PutData(fromData *bytes.Buffer, toPath string) (err error) {
	defer s.Connect.Close()

	// New Session
	session, err := s.Connect.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	// Read Dir or File
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		w.Write(fromData.Bytes())
	}()

	err = session.Run("/usr/bin/scp -ptr '" + toPath + "'")

	return
}

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
