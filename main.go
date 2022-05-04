package main

import (
	"io/ioutil"
	"os"
	"github.com/mft-labs/sftpmgr/sftpclient"
	"flag"
	"log"
)

var (
	conf string
	host string
	port string
	action string
	username string
	password string
	privatekey string
	srcpath string
	targetpath string
	usekey bool
	err error
	clean bool
	pattern string

)

func init() {
	//flag.StringVar(&conf,"conf","sftpmgr.conf","SFTP Mgr Config")
	flag.StringVar(&action,"action","upload","Action to be performed")
	flag.StringVar(&host,"host","localhost","Host")
	flag.StringVar(&port,"port","24039","Port")
	flag.StringVar(&username, "username","","Username")
	flag.StringVar(&password,"password","","Password")
	flag.StringVar(&privatekey,"privatekey","","Private Key")
	flag.StringVar(&srcpath,"src","","Source Path")
	flag.StringVar(&targetpath,"target","","Target Path")
	flag.BoolVar(&usekey,"usekey",false,"Use private key")
	flag.BoolVar(&clean,"clean", false, "Clean after upload/download")
	flag.StringVar(&pattern,"pattern","","File pattern matched for download")
}

func main() {
	flag.Parse()
	log.Printf("Running with action: %v",action)
	app := &sftpclient.SftpClient{}
	app.Host = host
	app.Port = port
	app.Username = username
	app.Path = targetpath
	app.UsePrivateKey = false
	app.CleanFiles = clean
	if usekey  || (password== "" && privatekey != ""){
		var contents = make([]byte,0)
		contents, err = ioutil.ReadFile(privatekey)
		if err!=nil {
			log.Printf("Failed to retrieve private key:%v",err)
			os.Exit(1)
		}
		app.PrivateKey = string(contents)
		//log.Printf("Using Key:\n%v",app.PrivateKey)
		app.UsePrivateKey = true
	} else {
		app.Password = password
	}

	//sftpmgr.exe -username User1 -privatekey sftpkeys/user1_amf -src testfiles  -action upload -clean
	if action == "upload" {
		app.FilesList, err = app.RetrieveFilesList(srcpath)
		if err!=nil {
			log.Printf("Error occurred while retrieving files list:%v",err)
		} else {
			if len(app.FilesList) == 0 {
				log.Printf("No files available for upload")
			} else {
				successfuldeliveries := 0
				failuredeliveries := 0
				log.Printf("Going to upload %v files",len(app.FilesList))
				for _, filename := range app.FilesList {
					err = app.UploadFile(filename)
					if err!=nil {
						failuredeliveries += 1
						log.Printf("Failed to upload:%v",filename)
					} else {
						successfuldeliveries += 1
						log.Printf("Uploaded %v successfully",filename)
					}
				}
				log.Printf("Successful deliveries:%v",successfuldeliveries)
				log.Printf("Deliveries with failure:%v",failuredeliveries)
			}
		}
	} else if action == "download" {
		//sftpmgr.exe -username User2 -privatekey sftpkeys/user1_amf -src downloads  -target /Inbox -action download -pattern *.html -clean
		if app.UsePrivateKey {
			err = app.ConnectWithPublicKey(app.Host, app.Port, app.Username, app.PrivateKey)
		} else {
			err = app.ConnectWithPassword(app.Host, app.Port, app.Username, app.Password)
		}
		if err == nil {
			app.TargetPath = srcpath
			app.FetchFiles(app.Path, pattern)
			//time.Sleep(time.Second*60)
			app.Close()
			log.Printf("Process for fetching of files completed")
		} else {
			log.Printf("Failed to connect to SFTP Server: %v:%v/%v reason %v",app.Host,app.Port,app.Username,err)
		}

	}
}
