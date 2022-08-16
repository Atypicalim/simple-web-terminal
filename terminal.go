// terminal

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
	"bytes"
	"flag"
    "net"
	"net/http"
	"os/exec"
	"path/filepath"
	"encoding/base64"
    "crypto/md5"
    "encoding/hex"
	"github.com/gorilla/websocket"
	"github.com/axgle/mahonia"
)

var (
    portPointer *int
    shellPointer  *string
    codePointer  *string
	passPointer *string
	staticPointer *bool
)

const TAG_ERR = "swt_error:"
const TAG_LOGIN = "swt_login:"

var isConnected bool = false
var isLogged bool = false
var isStatic bool = true
var serverPort int = 7777
var passwordHash string = ""
var userPassword string = ""
var userCoding string = ""
var userShell string = ""
var userDecoder mahonia.Decoder = nil

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var FORBIDDEN_COMMANDS = []string{
	"reboot",
	"poweroff",
	"ssh",
	"bash",
	"powershell",
	"sh",
}

var AVAILABLE_SHELLS = []string{
	"bash",
	"sh",
	"powershell",
}

func readId(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

func getMd5(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
 }

 const TERMIAL_CONTENT = `%s` // [M[ FILE_PLAIN | ./terminal.html ]M]

func getHelp() string {
	help := "\ncommands (%s):\n" // [M[ LINE_CALLBACK | VERSION_NAME ]M]
	help += "💁🏻 [ help ] print this help info again\n"
	help += "🙅‍♂️ [ stop ] stop current terminal program\n"
	help += "💾 [ save ] save current terminal content\n"
	help += "✂️ [ exit ] exit current websocket connection\n"
	help += "🔨 [ shell ] get/set current shell name\n"
	help += "👁️ [ coding ] get/set current encoding name\n"
	help += "📁 [ static ] open tab for static server\n"
	help += "🚃 [ scp ./ ] upload file to current directory\n"
	help += "🚋 [ scp ./test.txt ] download file from target path\n"
	help += "📝 [ vim ./test.txt ] edit file with simple editor\n"
	return help
}

func setShell(name string) bool {
	command := exec.Command(name, "-c", "ls")
	_, err := command.Output()
	if err == nil {
		userShell = name
		return true
	} else {
		return false
	}
}

func setCoding(name string) bool {
	decoder := mahonia.NewDecoder(name)
	if decoder != nil {
		userCoding = name
		userDecoder = decoder
		return true
	} else {
		return false
	}
}

func getPath(name string) string {
	path, err := os.Getwd()
	length := len(name)
	if err != nil {
		return ""
	}
	if (length == 1 && string(name[0]) == "~") || (length > 1 && string(name[1]) == ":") || string(name[0]) == "/" {
		path = name
	} else {
		path = filepath.Join(path, name)
	}
	return path
}

func changeDirectory(name string) string {
	dir := getPath(name)
	err := os.Chdir(dir)
	if err != nil {
		return err.Error()
	}
	dir, err = os.Getwd()
	if err != nil {
		return err.Error()
	}
	return dir
}

func executeCommand(input string, conn *websocket.Conn) string {
	log.Println("executing:" + input)
	args := strings.Fields(input)
	cmd := args[0]
	//
	if cmd == "login" {
		if len(args) != 2 {
			return TAG_ERR + "invalid argument count for " + cmd
		} else if isLogged {
			return TAG_ERR + "invalid state, try later!"
		} else if (passwordHash != args[1]) {
			return TAG_ERR + "invalid password, try again!"
		}
		isLogged = true
		return TAG_LOGIN + "finish"
	}
	// 
	for _, name := range FORBIDDEN_COMMANDS {
        if cmd == name {
			return TAG_ERR + "command not supported!"
        }
    }
	//
	if cmd == "exit" {
		conn.Close()
		return "exit!"
	}
	//
	if cmd == "help" {
		return getHelp()
	}
	//
	if cmd == "stop" {
		conn.WriteMessage(1, []byte("program stopping ..."))
		os.Exit(0)
		return "exit!"
	}
	//
	if cmd == "save" {
		return "swt_save:"
	}
	//
	if cmd == "shell" {
		if len(args) == 1 {
			return userShell
		} else if len(args) != 2 {
			return TAG_ERR + "invalid argument count for " + cmd
		}
		isOk := setShell(args[1])
		if isOk {
			return "current shell changed ..."
		} else {
			return TAG_ERR + "invalid encoding name ..."
		}
	}
	//
	if cmd == "coding" {
		if len(args) == 1 {
			return userCoding
		} else if len(args) != 2 {
			return TAG_ERR + "invalid argument count for " + cmd
		}
		isOk := setCoding(args[1])
		if isOk {
			return "current encoding changed ..."
		} else {
			return TAG_ERR + "invalid encoding name ..."
		}
	}
	//
	if cmd == "static" {
		if !isStatic {
			return TAG_ERR + "static server not started ..."
		}
		return "swt_static:"
	}
	//
	if cmd == "scp" {
		if len(args) != 2 {
			return TAG_ERR + "invalid argument count for " + cmd
		}
		path := args[1]
		path = getPath(path)
		info, err := os.Stat(path)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			return TAG_ERR + "path not exist"
		}
		flag := base64.StdEncoding.EncodeToString([]byte(path))
		if info.IsDir() {
			return "swt_upload:" + flag
		} else {
			return "swt_download:" + flag
		}
	}
	//
	if cmd == "vim" || cmd == "vi" {
		if len(args) != 2 {
			return TAG_ERR + "invalid argument count for " + cmd
		}
		path := args[1]
		path = getPath(path)
		info, err := os.Stat(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println(err)
			return err.Error()
		}
		if info != nil && info.IsDir() {
			return TAG_ERR + "cannot edit a directory!"
		}
		if info != nil && info.Size() > 1 * 1024 * 1024 {
			return TAG_ERR + "file too large, cannot edit!"
		}
		flag := base64.StdEncoding.EncodeToString([]byte(path))
		return "swt_edit:" + flag
	}
	//
	if cmd == "cd" || cmd == "chdir" {
		if len(args) != 2 {
			return TAG_ERR + "invalid argument count for " + cmd
		}
		return changeDirectory(args[1])
	}
	//
    command := exec.Command(userShell, "-c", input)
	var error bytes.Buffer
	command.Stderr = &error
	stdout, err := command.Output()
	if err != nil {
		return TAG_ERR + err.Error() + "," + userDecoder.ConvertString(error.String())
	}
	decoded := userDecoder.ConvertString(string(stdout))
	return strings.TrimSpace(decoded)
}

func handleTerminal(w http.ResponseWriter, r *http.Request) {
	log.Println("connecting:" + readId(r))
	time.Sleep(100 * time.Millisecond)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	if err != nil {
		log.Println(err)
		return
	}
	if isConnected {
		log.Println("refused!")
		conn.WriteMessage(1, []byte(TAG_ERR + "connecting with another terminal, please wait or close previous terminals!"))
		conn.Close()
		return
	} else {
		log.Println("connected!")
		isConnected = true
		isLogged = len(userPassword) == 0
	}
	if isLogged {
		conn.WriteMessage(1, []byte(TAG_LOGIN + "ignore"))
	} else {
		conn.WriteMessage(1, []byte(TAG_LOGIN + "start"))
	}
	for {
		messageType, bytes, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			isConnected = false
			isLogged = false
			return
		}
		input := string(bytes[:])
		output := executeCommand(input, conn)
		err = conn.WriteMessage(messageType, []byte(output))
		if err != nil {
			log.Println(err)
			isConnected = false
			isLogged = false
			return
		}
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	flag := r.URL.Query().Get("flag")
	arr,err := base64.StdEncoding.DecodeString(flag)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "err: flag not found ...")
		return
	}
	path := string(arr)
	info, err := os.Stat(path);
	if err != nil && errors.Is(err, os.ErrNotExist) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "err: file not found ...")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, err.Error())
	} else if info.IsDir() {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "err: cannot download a directory!")
	} else {
		log.Println("downloading:" + path)
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		fmt.Fprint(w, "upload...")
	} else if r.Method == "POST" {
		flag := r.URL.Query().Get("flag")
		arr,err := base64.StdEncoding.DecodeString(flag)
		if err != nil {
			fmt.Fprint(w, "err: flag not found ...")
			return
		}
		path := string(arr)
		info, _ := os.Stat(path);
		if err != nil && errors.Is(err, os.ErrNotExist) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "err: directory not found ...")
			return
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, err.Error())
			return
		} else if !info.IsDir() {
			fmt.Fprint(w, "err: cannot upload to file!")
			return 
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10 * 1024 * 1024)
		file, handler, err := r.FormFile("fileUpload")
		if err != nil {
			log.Println(err)
			fmt.Fprint(w, err.Error())
			return
		}
		defer file.Close()
		log.Println("uploading:" + handler.Filename)
		target, err := os.OpenFile(path + "/" + handler.Filename, os.O_WRONLY | os.O_CREATE, 0777)
		if err != nil {
			log.Println(err)
			fmt.Fprint(w, err.Error())
			return
		}
		defer target.Close()
		_, err = io.Copy(target, file)
		if err != nil {
			log.Println(err)
			fmt.Fprint(w, err.Error())
			return
		}
		log.Println("uploaded:" + handler.Filename)
		fmt.Fprint(w, "successful...")
	}
}

func handleEdit(w http.ResponseWriter, r *http.Request) {
	flag := r.URL.Query().Get("flag")
	arr,err := base64.StdEncoding.DecodeString(flag)
	if err != nil {
		fmt.Fprint(w, "err: flag not found ...")
		return
	}
	path := string(arr)
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println(err)
		fmt.Fprint(w, err.Error())
		return
	}
	if info != nil && info.IsDir() {
		fmt.Fprint(w, "err: cannot edit a directory!")
		return 
	}
	if r.Method == "GET" {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprint(w, "")
		} else {
			http.ServeFile(w, r, path)
		}
	} else if r.Method == "POST" {
		if err := r.ParseMultipartForm(0); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		log.Println("editing:" + path)
		text := r.FormValue("textEdit")
		target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		defer target.Close()
		if _, err = target.WriteString(text); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		log.Println("edited:" + path)
		fmt.Fprint(w, "saved!")
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Println("responsing:" + readId(r))
	time.Sleep(100 * time.Microsecond)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "terminal.html") // [M[ LINE_REFPLACE | fmt.Fprint(w, TERMIAL_CONTENT) ]M]
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func parseArgs() {
    flag.Parse()
	serverPort = *portPointer;
	shellName := *shellPointer;
	codingName := *codePointer;
	passValue := *passPointer;
	isStatic = *staticPointer;
	//
	if len(userShell) == 0 {
		setShell(shellName)
	}
	for _, name := range AVAILABLE_SHELLS {
		if len(userShell) == 0 { setShell(name) }
	}
	//
	if len(codingName) > 0 {
		setCoding(codingName)
	}
	if len(userCoding) == 0 {
		setCoding("utf8")
	}
	//
	if len(passValue) > 0 {
		userPassword = passValue
	}
	regexPattern, _ := regexp.Compile(`\s+`)
	userPassword = regexPattern.ReplaceAllString(strings.TrimSpace(userPassword), "")
	passwordHash = getMd5(userPassword)
}

func init() {
    portPointer = flag.Int("port", serverPort, "terminal serving port")
    shellPointer = flag.String("shell", userShell, "set default shell")
    codePointer = flag.String("coding", userCoding, "set output encoding")
    passPointer = flag.String("pass", userPassword, "set terminal password")
    staticPointer = flag.Bool("static", isStatic, "start static server")
}

func main() {
	fmt.Println("")
	log.Println("♚  Welcome To Simple Web Terminal!")
	parseArgs()
	log.Println(fmt.Sprintf("☛  pass: [%s]", userPassword))
	log.Println("☛  date:", time.Now())
	log.Println("☛  plat:", runtime.GOOS)
	log.Println("☛  arch:", runtime.GOARCH)
	log.Println("☛  port:", serverPort)
	log.Println("☛  shell:", userShell)
	log.Println("☛  coding:", userCoding)
	//
	if isStatic {
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./"))))
	}
	//
	http.HandleFunc("/terminal", handleTerminal)
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/edit", handleEdit)
	http.HandleFunc("/", handleRequest)
	//
	ip := "127.0.0.1"
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err == nil {
		ip = conn.LocalAddr().(*net.UDPAddr).IP.String()
    }
    conn.Close()
	//
	log.Println("☛  addr:", fmt.Sprintf("http:%s:%d", ip, serverPort))
	log.Println("☛  addr:", fmt.Sprintf("http:127.0.0.1:%d", serverPort))
	log.Println("running...")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil))
}
