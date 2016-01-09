package lobster

import "github.com/gorilla/websocket"

import "bufio"
import "encoding/base64"
import "fmt"
import "net"
import "net/http"
import "strings"
import "testing"
import "time"

func TestWebsockify(t *testing.T) {
	TestReset()

	// create fake listener
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	okChannel := make(chan bool, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Error(err)
			okChannel <- false
			return
		}
		defer conn.Close()
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Errorf("Server SetWriteDeadline: %v", err)
		}
		message, err := bufio.NewReader(conn).ReadString('\n')
		message = strings.TrimSpace(message)

		if err != nil {
			t.Errorf("Server Read: %v", err)
			okChannel <- false
			return
		} else if message != "ping" {
			t.Errorf("Expected ping, got %s", message)
			okChannel <- false
			return
		}

		if _, err := conn.Write([]byte("pong\n")); err != nil {
			t.Errorf("Server Write: %v", err)
		}
		okChannel <- true
	}()

	defer func() {
		ln.Close()
		lnOk := <-okChannel
		if !lnOk {
			t.Errorf("Listener not OK")
		}
	}()

	// connect via websocket
	// HandleWebsockify returns only the token with the testing Novnc.Url configuration
	token := HandleWebsockify(fmt.Sprintf("127.0.0.1:%d", port), "foo")
	ws, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:6080/websockify", http.Header{"Cookie": {"token=" + token}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, []byte(base64.StdEncoding.EncodeToString([]byte("ping\n")))); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if err := ws.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, messageBase64, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	message, _ := base64.StdEncoding.DecodeString(string(messageBase64))
	if strings.TrimSpace(string(message)) != "pong" {
		t.Fatalf("Expected pong, got %s", message)
	}
}
