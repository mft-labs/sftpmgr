package sftpclient

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	//"github.com/google/uuid"
	"io"
	"log"
	"net"
	"os"
	"path"
	//"path/filepath"
	"runtime"
	"strings"
	"time"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

)


type SftpClient struct {
	Host string
	Port string
	Username string
	Password string
	PrivateKey string
	Conn *ssh.Client
	Client *sftp.Client
	Path string
	UsePrivateKey bool
	FilesList []string
	Interval int
	DelayUnits string
	CleanFiles bool
	TargetPath string
}

func (s *SftpClient) UploadFile(srcfile string) error {
		now := time.Now()
		srcfile2 := strings.Replace(srcfile,"\\","/",-1)
		filename := fmt.Sprintf("%v/%v",s.Path,path.Base(srcfile2))
		log.Printf("Uploading target file:%v at %v",filename,now)
		err := s.PutFile(srcfile,filename)
		if err==nil {
			if s.CleanFiles  {
				err = os.Remove(srcfile)
				if err!=nil {
					log.Printf("Failed to clean file %v reason: %v",srcfile, err)
					return err
				}
			}
		} else {
			log.Printf("Failed to upload file %v reason:%v",srcfile, err)
			return err
		}
		if s.DelayUnits == "Seconds" {
			time.Sleep(time.Second*time.Duration(s.Interval))
		} else if s.DelayUnits == "MilliSeconds" {
			time.Sleep(time.Millisecond*time.Duration(s.Interval))
		} else if s.DelayUnits == "MicroSeconds" {
			time.Sleep(time.Microsecond*time.Duration(s.Interval))
		} else if s.DelayUnits == "NanoSeconds" {
			time.Sleep(time.Nanosecond*time.Duration(s.Interval))
		}
		return nil
}

func  (s *SftpClient)  GetPublicKey(privkey string) ssh.AuthMethod {
	var buf bytes.Buffer
	buf.Write([]byte(privkey))
	key, err := ssh.ParsePrivateKey(buf.Bytes())
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func (s *SftpClient)  ConnectWithPublicKey(host, port, username, privkey string) error {
	var err error
	log.Printf("Connecting with Private key: %v:%v/%v",host,port,username)
	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			s.GetPublicKey(privkey),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
	}
	s.Conn, err = ssh.Dial("tcp", fmt.Sprintf("%v:%v",host,port), sshConfig)
	if err != nil {
		return  fmt.Errorf("Failed to dial(1): %s", err)
	}
	s.Client, err = sftp.NewClient(s.Conn)
	if err != nil {
		log.Printf("Connecting with public key failed:%v",err)
		return err
	}
	return nil
}

func (s *SftpClient)  ConnectWithPassword(host, port, username, password string) error {
	var err error
	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
	}
	s.Conn, err = ssh.Dial("tcp", fmt.Sprintf("%v:%v",host,port), sshConfig)
	if err != nil {
		return  fmt.Errorf("Failed to dial(2): %s", err)
	}
	s.Client, err = sftp.NewClient(s.Conn)
	if err != nil {
		return err
	}
	return nil
}

func (s *SftpClient) Close() {
	if s.Client!=nil {
		s.Client.Close()
		if s.Conn!=nil {
			s.Conn.Close()
		}
	}
}

func (s *SftpClient) PutFile(srcFile, filename string) error {
	var err error
	if s.UsePrivateKey {
		log.Printf("Connecting with Private Key")
		err = s.ConnectWithPublicKey(s.Host,s.Port, s.Username,s.PrivateKey)
	} else {
		log.Printf("Connecting with Password")
		err = s.ConnectWithPassword(s.Host,s.Port, s.Username,s.Password)
	}
	if err != nil {
		log.Printf("SFTP Connection failed to establish:%v",err)
		return err
	}

	log.Printf("Uploading source file:%v",srcFile)
	fr, ferr := os.Open(srcFile)
	if ferr!=nil {
		log.Printf("Failed to retrieve source file:%v",srcFile)
		return ferr
	}
	//defer fr.Close()
	if filename[0] == '/' {
		filename = filename[1:]
	}
	fw, ferr2 := s.Client.Create(filename)
	if ferr2 != nil {
		fr.Close()
		log.Printf("Failed to create target file  on server:%v",filename)
		return ferr2
	}
	//fw.Close()
	n, cErr := io.Copy(fw, fr)
	if cErr!=nil {
		fr.Close()
		fw.Close()
		log.Printf("Failed to upload file %v to SFTP Server:%v",filename,s.Host)
		return cErr
	}
	fr.Close()
	fw.Close()
	log.Printf("Successfully uploaded file:%v with Size:%v",filename,n)
	s.Close()
	return nil
}

func (s *SftpClient) MatchPattern(pattern,samplePattern string) (bool,error){
	matched, err := path.Match(pattern,samplePattern)
	if err != nil{
		return false,err
	}
	if matched {
		return true,nil
	} else {
		return false,nil
	}
}

func (s *SftpClient) ProcessFile(src string) error {
	current := time.Now()
	y, m, d := current.Date()
	mon := m.String()[:3]
	mon = strings.ToLower(mon)
	dir := fmt.Sprintf("%d/%s/%02d/%s", y, mon, d,s.Username)
	//str, _ := uuid.NewUUID()
	//msgid := str.String()
	dst2 := s.TargetPath+"/"+dir+"/"+path.Base(src)
	sep := "/"
	if runtime.GOOS == "windows" {
		dst2 = strings.Replace(dst2,"/","\\",-1)
		dir = strings.Replace(dir,"/","\\",-1)
		sep = "\\"
	}
	log.Printf("Going to create file in the path:%v",dst2)
	if _, err := os.Stat(s.TargetPath+sep+dir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(s.TargetPath+sep+dir, os.ModePerm)
		if err != nil {
			log.Printf("Failed to create target folder:%v, Reason:%v\n",s.TargetPath+sep+dir,err)
			return err
		}
	}
	dstFile, err := os.Create(dst2)
	if err != nil {
		fmt.Printf("Failed to open target file:%v reason:%v\n",dst2,err)
		return err
	}
	//defer dstFile.Close()
	srcFile, err := s.Client.Open(src)
	if err != nil {
		dstFile.Close()
		fmt.Printf("Failed to retrieve source file:%v reason %v\n",src,err)
		return err
	}
	//defer srcFile.Close()
	// copy source file to destination file
	byteCount, err := io.Copy(dstFile, srcFile)
	if err != nil {
		dstFile.Close()
		srcFile.Close()
		fmt.Printf("Failed to copy source %v to target %v, reason:%v\n",src, dst2,err)
		return err
	}
	fmt.Printf("%d bytes copied\n", byteCount)

	// flush in-memory copy
	err = dstFile.Sync()
	if err != nil {
		dstFile.Close()
		srcFile.Close()
		return err
	}

	return nil
}

func (s *SftpClient) FetchFiles(remoteDir, filePattern string) {
	fmt.Printf("Getting list of available files at %v and checking for pattern %v\n",remoteDir, filePattern)
	w := s.Client.Walk(remoteDir)
	for w.Step() {
		log.Printf("Checking file : %v",w.Path())
		if w.Err() != nil {
			continue
		}
		pattern := strings.Replace(filepath.Join(remoteDir,filePattern),"\\","/",-1)
		fmt.Printf("Going to compare %v with %v\n",pattern,w.Path())
		if s.Path == w.Path() {
			continue
		}
		if len(w.Path()) <= len(s.Path) {
			log.Printf("Skipping directory:%v",w.Path())
			continue
		}
		matched, err := s.MatchPattern(pattern,w.Path())
		if err!=nil {
			log.Printf("Error occurred while match with files:%v",err)
			continue
		}
		if !matched {
			log.Printf("Skipping file %v",w.Path())
			continue
		}
		if matched {
			log.Println(w.Path() + " Matched with "+s.Path)
			if w.Path() == s.Path {
				continue
			}
			err = s.ProcessFile(w.Path())
			if err!=nil {
				fmt.Printf("Error occurred while processing file:%v\n",err)
			} else {
				if s.CleanFiles {
					//err = s.Client.Remove(w.Path())
					err = s.Client.Remove(path.Join(s.Path,path.Base(w.Path())))
					if err!=nil {
						fmt.Printf("Failed to cleanup file:%v\n",err)
						continue
					} else {
						fmt.Printf("Removed file:%v successfully from server\n",path.Join(s.Path,path.Base(w.Path())))
					}
				}
				//time.Sleep(time.Second)
			}
		} else {
			log.Println(w.Path() + " Not Matched")
		}

	}
}