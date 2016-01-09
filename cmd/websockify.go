package main

import "github.com/LunaNode/lobster/websockify"

import "fmt"

func main() {
	ws := websockify.Websockify{
		Debug: true,
	}
	ws.Run()
	fmt.Println(ws.Register("127.0.0.1:5900"))
	select {}
}
