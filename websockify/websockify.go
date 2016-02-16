package websockify

import "github.com/gorilla/websocket"

import "crypto/rand"
import "encoding/base64"
import "log"
import "net"
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
		str[i] = alphabet[int(bytes[i])%len(alphabet)]
	}
	return string(str)
}

type tokenTarget struct {
	ipport string // e.g. 127.0.0.1:5900
	time   time.Time
}

type Websockify struct {
	// Location to listen for connections, defaults to ":6080"
	Listen string

	// Whether to print debug output
	Debug bool

	tokens     map[string]*tokenTarget
	upgrader   websocket.Upgrader
	fileserver http.Handler
}

func (this *Websockify) Run() {
	if this.Listen == "" {
		this.Listen = ":6080"
	}

	this.tokens = make(map[string]*tokenTarget)
	this.upgrader = websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
	}
	this.fileserver = http.FileServer(http.Dir("./novnc/"))

	httpServer := &http.Server{
		Addr:    this.Listen,
		Handler: this,
	}

	go func() {
		log.Fatal(httpServer.ListenAndServe())
	}()
}

func (this *Websockify) Register(ipport string) string {
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
		time:   time.Now(),
	}

	return token
}

func (this *Websockify) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/websockify" {
		this.fileserver.ServeHTTP(w, r)
		return
	}

	// check token
	tokenCookie, err := r.Cookie("token")
	if err != nil {
		if this.Debug {
			log.Printf("Token cookie error (%s): %s", r.RemoteAddr, err.Error())
		}
		return
	}
	target := this.tokens[tokenCookie.Value]
	if target == nil {
		if this.Debug {
			log.Printf("Invalid token (%s): %s", r.RemoteAddr, tokenCookie.Value)
		}
		return
	}

	// try upgrade connection
	responseHeader := make(http.Header)
	requestedSubprotocols := websocket.Subprotocols(r)
	if len(requestedSubprotocols) > 0 {
		// pick base64 subprotocol if available
		// otherwise arbitrarily pick the first one and hope for the best
		pickedSubprotocol := ""
		for _, subprotocol := range requestedSubprotocols {
			if subprotocol == "base64" {
				pickedSubprotocol = "base64"
			}
		}
		if pickedSubprotocol == "" {
			pickedSubprotocol = requestedSubprotocols[0]
		}
		responseHeader.Set("Sec-Websocket-Protocol", pickedSubprotocol)
	}

	conn, err := this.upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		log.Printf("websockify error (%s): %s", r.RemoteAddr, err.Error())
		return
	}
	defer conn.Close()

	if this.Debug {
		log.Printf("Initializing connection from %s to %s", r.RemoteAddr, target.ipport)
	}

	sock, err := net.Dial("tcp", target.ipport)
	if err != nil {
		if this.Debug {
			log.Print(err)
		}
		return
	}
	defer sock.Close()

	done := make(chan bool, 2)
	go func() {
		defer func() {
			done <- true
		}()
		wbuf := make([]byte, 32*1024)
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if messageType == websocket.TextMessage {
				n, _ := base64.StdEncoding.Decode(wbuf, p)
				_, err = sock.Write(wbuf[:n])
				if err != nil {
					return
				}
			}
		}
	}()
	go func() {
		defer func() {
			done <- true
		}()
		rbuf := make([]byte, 8192)
		wbuf := make([]byte, len(rbuf)*2)
		for {
			n, err := sock.Read(rbuf)
			if err != nil {
				return
			}

			if n > 0 {
				base64.StdEncoding.Encode(wbuf, rbuf[:n])
				err = conn.WriteMessage(websocket.TextMessage, wbuf[:base64.StdEncoding.EncodedLen(n)])
				if err != nil {
					return
				}
			}
		}
	}()
	<-done
}
