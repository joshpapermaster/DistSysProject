package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

type CommandMsg struct {
  Action string `json:"action"`
  Audience string `json:"audience"`
  Type string `json:"type"`
  Value interface{} `json:"value"`
}

type Resp struct {
	Type string
	Value []string
}
var addr = flag.String("addr", "osavxvy2eg.execute-api.us-east-1.amazonaws.com", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/dev"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	aud := "all"

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			// aud = message["value"]
			var response Resp
			json.Unmarshal([]byte(message), &response)
			log.Printf("recv: %s", message)
		}
	}()

	temp := &CommandMsg{Action: "onMessage", Audience: aud, Type: "Command", Value: "5"}
  err = c.WriteJSON(temp)
  if err != nil {
    log.Println("write:", err)
    return
  }
	go func() {
		time.Sleep(5000 * time.Millisecond)
		temp := &CommandMsg{Action: "onMessage", Audience: aud, Type: "KillServerI", Value: []string{"1"}}
	  err = c.WriteJSON(temp)
	  if err != nil {
	    log.Println("write:", err)
	    return
	  }
	}()
	for {
    select {
		case <-done:
			return
    case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
			}
      return
    }
  }
}
