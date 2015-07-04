package wssh

import "github.com/gorilla/websocket"
import "golang.org/x/crypto/ssh"

import "crypto/rand"
import "encoding/json"
import "log"
import "net/http"
import "time"

func newToken(l int) string {
	alphabet := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	bytes := make([]byte, l)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	str := make([]rune, len(bytes))
	for i := range bytes {
		str[i] = alphabet[int(bytes[i]) % len(alphabet)]
	}
	return string(str)
}

type tokenTarget struct {
	ipport string
	username string
	password string
	time time.Time
}

type Wssh struct {
	// Location to listen for connections, defaults to ":7080"
	Listen string

	// Whether to print debug output
	Debug bool

	tokens map[string]*tokenTarget
	upgrader websocket.Upgrader
	fileserver http.Handler
}

func (this *Wssh) Run() {
	if this.Listen == "" {
		this.Listen = ":7080"
	}

	this.tokens = make(map[string]*tokenTarget)
	this.upgrader = websocket.Upgrader{
		ReadBufferSize: 2048,
		WriteBufferSize: 2048,
	}
	this.fileserver = http.FileServer(http.Dir("./wssh/assets/"))

	httpServer := &http.Server{
		Addr: this.Listen,
		Handler: this,
	}

	go func() {
		log.Fatal(httpServer.ListenAndServe())
	}()
}

func (this *Wssh) Register(ipport string, username string, password string) string {
	// delete old tokens
	for token, target := range this.tokens {
		if time.Now().Sub(target.time) > time.Hour {
			delete(this.tokens, token)
		}
	}

	// insert
	token := newToken(32)
	this.tokens[token] = &tokenTarget{
		ipport: ipport,
		username: username,
		password: password,
		time: time.Now(),
	}

	return token
}

type ptyRequestMsg struct {
	Term     string
	Columns  uint32
	Rows     uint32
	Width    uint32
	Height   uint32
	Modelist string
}

type clientResize struct {
	Width int `json:"width"`
	Height int `json:"height"`
}
type clientMessage struct {
	Data string `json:"data"`
	Resize *clientResize `json:"resize"`
}

func (this *Wssh) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wssh" {
		this.fileserver.ServeHTTP(w, r)
		return
	}

	// check token
	tokenCookie, err := r.Cookie("token")
	if err != nil {
		if this.Debug {
			log.Printf("wssh: token cookie error (%s): %s", r.RemoteAddr, err.Error())
		}
		return
	}
	target := this.tokens[tokenCookie.Value]
	if target == nil {
		if this.Debug {
			log.Printf("wssh: invaild token (%s): %s", r.RemoteAddr, tokenCookie.Value)
		}
		return
	}

	// try upgrade connection
	conn, err := this.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("wssh error (%s): %s", r.RemoteAddr, err.Error())
		return
	}
	defer conn.Close()

	// make ssh connection
	if this.Debug {
		log.Printf("wssh: initializing connection from %s to %s@%s", r.RemoteAddr, target.username, target.ipport)
	}
	config := &ssh.ClientConfig{
		User: target.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(target.password),
		},
	}
	client, err := ssh.Dial("tcp", target.ipport, config)
	if err != nil {
		if this.Debug {
			log.Print(err)
		}
		return
	}
	defer client.Close()

	// open shell channel
	channel, incomingRequests, err := client.Conn.OpenChannel("session", nil)
	if err != nil {
		if this.Debug {
			log.Print(err)
		}
		return
	}
	go func() {
		for req := range incomingRequests {
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}()
	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	var tm []byte
	for k, v := range modes {
		kv := struct {
			Key byte
			Val uint32
		}{k, v}
		tm = append(tm, ssh.Marshal(&kv)...)
	}
	tm = append(tm, 0)
	req := ptyRequestMsg{
		Term:     "xterm",
		Columns:  80,
		Rows:     40,
		Width:    80 * 8,
		Height:   40 * 8,
		Modelist: string(tm),
	}
	ok, err := channel.SendRequest("pty-req", true, ssh.Marshal(&req))
	if !ok || err != nil {
		log.Printf("wssh fail to do pty-req %v", err)
		return
	}
	ok, err = channel.SendRequest("shell", true, nil)
	if !ok || err != nil {
		log.Printf("wssh fail to do shell %v", err)
		return
	}

	done := make(chan bool, 2)
	go func() {
		defer func() {
			done <- true
		}()
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if messageType == websocket.TextMessage {
				messageStruct := new(clientMessage)
				json.Unmarshal(p, messageStruct)
				if messageStruct.Data != "" {
					_, err := channel.Write([]byte(messageStruct.Data))
					if err != nil {
						return
					}
				}
			}
		}
	}()
	go func() {
		defer func() {
			done <- true
		}()
		rbuf := make([]byte, 1024)
		for {
			n, err := channel.Read(rbuf)
			if err != nil {
				return
			}

			if n > 0 {
				msg := &clientMessage {
					Data: string(rbuf[:n]),
				}
				msgBytes, err := json.Marshal(msg)
				err = conn.WriteMessage(websocket.TextMessage, msgBytes)
				if err != nil {
					return
				}
			}
		}
	}()
	<- done
}
