package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"sync/atomic"

	// "time"

	"github.com/gorilla/websocket"
)

type Msg struct {
	Action   string   `json:"action"`
	Audience string   `json:"audience"`
	Type     string   `json:"type"`
	Value    []string `json:"value"`
}

type Resp struct {
	Type  string
	Value []string
}

type VoteReqResp struct {
	Type  string
	Value RequestVoteArgs
}

type VoteReplyResp struct {
	Type  string
	Value RequestVoteReply
}

type AppendEntriesResp struct {
	Type  string
	Value AppendEntriesArgs
}

type AppendEntriesReplyResp struct {
	Type  string
	Value AppendEntriesReply
}

type CommandReq struct {
	Type  string
	Value interface{}
}

var addr = flag.String("addr", "osavxvy2eg.execute-api.us-east-1.amazonaws.com", "http service address")
var unique_id = "2"
var unique_id_int = 2
var numServers = 3
var N = 3 // N = Max number of servers allowed in the configuration
var rf *Raft
var CurrentVoteReply RequestVoteReply
var CurrentHBReply AppendEntriesReply

func begin(serverMap []string, c *websocket.Conn) {
	persister := MakePersister()
	// rf := Make(serverMap, unique_id_int, persister, c)
	rf = Make(serverMap, unique_id_int, persister, c, N)
}

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
		serversFound := 0
		serverMap := make([]string, numServers)
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

			if response.Type == "Introduction" {
				i, _ := strconv.Atoi(response.Value[0])
				// Check if it's a new server
				if len(serverMap[i]) == 0 {
					serversFound++
					// New introduction so must introduce yourself
					if response.Value[0] != unique_id {
						temp := &Msg{Action: "onMessage", Audience: aud, Type: "Introduction", Value: []string{unique_id}}
						err := c.WriteJSON(temp)
						if err != nil {
							log.Println("write:", err)
							return
						}
					}
					if serversFound == numServers {
						// Ready to begin!
						go begin(serverMap, c)
						fmt.Println(reflect.TypeOf(c))

					}
				}
				// Regardless, overwrite the old configID value
				serverMap[i] = response.Value[1]
				log.Printf("srvmap: %s", serverMap)

			} else if response.Type == "RequestVote" {
				var response VoteReqResp
				json.Unmarshal([]byte(message), &response)
				// call RequestVoteReply
				go rf.RequestVote(&response.Value)

			} else if response.Type == "RequestVoteReply" {
				var response VoteReplyResp
				json.Unmarshal([]byte(message), &response)
				CurrentVoteReply.Term = response.Value.Term
				CurrentVoteReply.VoteGranted = response.Value.VoteGranted
				CurrentVoteReply.Me = response.Value.Me
				atomic.StoreInt32(&AwaitingVote, 0)
				log.Printf("Receving reply and setting awaitingVote to false")

			} else if response.Type == "Heartbeat" {
				log.Printf("Receving heartbeat")
				var response AppendEntriesResp
				json.Unmarshal([]byte(message), &response)
				go rf.AppendEntries(&response.Value)

			} else if response.Type == "AppendEntriesReply" {
				var response AppendEntriesReplyResp
				json.Unmarshal([]byte(message), &response)
				CurrentHBReply.Term = response.Value.Term
				CurrentHBReply.Success = response.Value.Success
				CurrentHBReply.RetryIndex = response.Value.RetryIndex
				atomic.StoreInt32(&AwaitingHB, 0)
				log.Printf("Receving HB Response and setting awaitingHB to false")

			} else if response.Type == "Command" {
				var response CommandReq
				json.Unmarshal([]byte(message), &response)
				go rf.Start(response.Value)

			} else if response.Type == "Kill" {
				go rf.Kill()

			}

		}
	}()

	temp := &Msg{Action: "onMessage", Audience: aud, Type: "Introduction", Value: []string{unique_id}}
	err = c.WriteJSON(temp)
	if err != nil {
		log.Println("write:", err)
		return
	}

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
