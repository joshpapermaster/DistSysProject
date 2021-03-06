package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"encoding/json"
	"strconv"
	"sync/atomic"

  // "time"

	"github.com/gorilla/websocket"
)

type Msg struct {
  Action string `json:"action"`
  Audience string `json:"audience"`
  Type string `json:"type"`
  Value []string `json:"value"`
}

type Resp struct {
	Type string
	Value []string
}

type VoteReqResp struct {
	Type string
	Value RequestVoteArgs
}

type VoteReplyResp struct {
	Type string
	Value RequestVoteReply
}

type AppendEntriesResp struct {
	Type string
	Value AppendEntriesArgs
}

type AppendEntriesReplyResp struct {
	Type string
	Value AppendEntriesReply
}

type CommandReq struct {
	Type string
	Value interface{}
}


var addr = flag.String("addr", "osavxvy2eg.execute-api.us-east-1.amazonaws.com", "http service address")
var unique_id = "1"
var unique_id_int = 1
var numServers = 4
var N = 3 // N = Max number of servers allowed in the configuration
var config = make(map[int]bool)
var rf *Raft
var CurrentVoteReply RequestVoteReply
var CurrentHBReply AppendEntriesReply

func begin(serverMap []string, c *websocket.Conn) {
  persister := MakePersister()
	rf = Make(serverMap, unique_id_int, persister, c, N, config)
}

func main() {
	// INITIALIZE WHICH SERVERS IN INITIAL Config
	// MUST BE CONSISTENT ACROSS ALL SERVERS
	// ONLY INITIALIZE TRUE VALUES
	config[0] = true
	config[1] = true
	config[2] = true


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
          if serversFound == numServers && config[unique_id_int] {
            // Ready to begin!
            go begin(serverMap, c)

          }
        }
        // Regardless, overwrite the old configID value
        serverMap[i] = response.Value[1]
  			log.Printf("srvmap: %s", serverMap)

      } else if response.Type=="RequestVote"{
				var response VoteReqResp
				json.Unmarshal([]byte(message), &response)
				// call RequestVoteReply
				if rf != nil {
					go rf.RequestVote(&response.Value)
				}


			} else if response.Type=="RequestVoteReply"{
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
				if rf != nil {
					go rf.AppendEntries(&response.Value)
				}

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
				if rf != nil {
					go rf.Start(response.Value)
				}

			} else if response.Type == "AliveCheck" {
				i, _ := strconv.Atoi(response.Value[0])
				if (rf != nil && rf.Killed()) {
					continue
				} else {
					temp := &Msg{Action: "onMessage", Audience: serverMap[i], Type: "AliveConfirm", Value: []string{unique_id}}
				  err = c.WriteJSON(temp)
				  if err != nil {
				    log.Println("write:", err)
				    return
				  }
				}

			} else if response.Type == "AliveConfirm" {
				i, _ := strconv.Atoi(response.Value[0])
				if rf != nil {
					go rf.AddServer(i)
				}

			} else if response.Type == "AddServer" {
				// RESET config to be empty so it can be overwritten during heartbeat
				config = make(map[int]bool)
				go begin(serverMap, c)

			} else if response.Type == "Kill" {
				go rf.Kill()

			} else if response.Type == "KillServerI" {
				i, _ := strconv.Atoi(response.Value[0])
				if i == unique_id_int {
					go rf.Kill()
				}
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
