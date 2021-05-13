package main

// Josh Papermaster
// jbp2463
//
//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import "sync/atomic"
import "math/rand"
import "time"
import "log"
import "bytes"
import "strconv"

import	"github.com/gorilla/websocket"


//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type LogEntry struct {
	Command interface{}
	CommitTerm int
}

type State int

const (
	Follower = iota
	Candidate
	Leader
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []string // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()
	currentTerm  int
	votedFor  int // -1 if none
	log				[]LogEntry
	commitIndex int
	lastApplied int
	nextIndex []int
	matchIndex []int
	config     map[int]bool
	configNum  int
	aliveCheck []int
	state			State
	c         *websocket.Conn
	N 				 int          // The max number of servers allowed to be alive
	numMissedElections int


	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

}

var AwaitingVote int32
var AwaitingHB int32

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	rf.mu.Lock()
	term = rf.currentTerm
	isleader = rf.state == Leader
	rf.mu.Unlock()
	// Your code here (2A).
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	w := new(bytes.Buffer)
	e := NewEncoder(w)
	e.Encode(&rf.currentTerm)
	e.Encode(&rf.votedFor)
	e.Encode(&rf.log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	r := bytes.NewBuffer(data)
	d := NewDecoder(r)
	d.Decode(&rf.currentTerm)
	log.Printf("Me: %d PERSISTED term: %d", rf.me, rf.currentTerm)
	d.Decode(&rf.votedFor)
	log.Printf("Me: %d PERSISTED vote: %d", rf.me, rf.votedFor)
	d.Decode(&rf.log)
	log.Printf("Me: %d PERSISTED log of length: %d", rf.me, len(rf.log))
}


//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term 	int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
	Config     map[int]bool
	ConfigNum  int
	Me int
}

type RequestVoteMsg struct {
  Action string `json:"action"`
  Audience string `json:"audience"`
  Type string `json:"type"`
  Value *RequestVoteArgs `json:"value"`
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term int
	VoteGranted bool
	Me int
}

type RequestVoteReplyMsg struct {
  Action string `json:"action"`
  Audience string `json:"audience"`
  Type string `json:"type"`
  Value *RequestVoteReply `json:"value"`
}


type AppendEntriesArgs struct {
	Term int // leader's term
	LeaderId int  // So followers can redirect clients
	PrevLogIndex int // index of log index immediately preceding new ones
	PrevLogTerm int // term of prev log index entry
	Entries []LogEntry // log entries to be stored (empty for hb)
	Config     map[int]bool
	ConfigNum  int
	LeaderCommit int  // leader's commit index
}

type AppendEntriesArgsMsg struct {
	Action string `json:"action"`
  Audience string `json:"audience"`
  Type string `json:"type"`
  Value *AppendEntriesArgs `json:"value"`
}

type AppendEntriesReply struct {
	Term int // current term (for the leader to update itself)
	Success bool // true if follower contained prev log term and prev log index
	RetryIndex int // index to retry with if failed
}

type AppendEntriesReplyMsg struct {
	Action string `json:"action"`
  Audience string `json:"audience"`
  Type string `json:"type"`
  Value *AppendEntriesReply `json:"value"`
}


func sendAppendEntriesReply(server int, reply *AppendEntriesReply, c *websocket.Conn) {
	log.Printf("Sending Heartbeat reply")
	temp := &AppendEntriesReplyMsg{Action: "onMessage", Audience: rf.peers[server], Type: "AppendEntriesReply", Value: reply}
	err := c.WriteJSON(temp)
  if err != nil {
    log.Println("write:", err)
    return
  }
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs) {
	var reply AppendEntriesReply
	rf.mu.Lock()
	defer rf.mu.Unlock()
	log.Printf("Me: %d RECEIVED HEARTBEAT", rf.me)
	reply.Term = rf.currentTerm

	if (args.Term < rf.currentTerm) {
		log.Printf("Me: %d received outdated leader", rf.me)
		sendAppendEntriesReply(args.LeaderId, &reply, rf.c)
		return
	} else if len(rf.log) <= args.PrevLogIndex || (args.PrevLogIndex >= 0 && rf.log[args.PrevLogIndex].CommitTerm != args.PrevLogTerm) {
		log.Printf("Me: %d received invalid PrevLogIndex or wrong entry term", rf.me)
		if len(rf.log) >  args.PrevLogIndex {
			log.Printf("Args term: %d my term: %d", args.PrevLogTerm,  rf.log[args.PrevLogIndex].CommitTerm )
		}

		log.Printf("Length of log: %d", len(rf.log))
		// log.Printf("PrevLogIndex: %d entry term: inserted term: %d, prevTerm: %d", args.PrevLogIndex, rf.log[args.PrevLogIndex].CommitTerm, args.PrevLogTerm)
		rf.state = Follower
		rf.currentTerm = args.Term
		rf.numMissedElections = 0
		rf.config = args.Config
		rf.configNum = args.ConfigNum
		rf.persist()
		reply.Success = false
		// decrement to find change in term
		reply.RetryIndex = len(rf.log)
		if len(rf.log) >  args.PrevLogIndex {
			for i:=args.PrevLogIndex; i>=0; i-- {
				if rf.log[i].CommitTerm != rf.log[args.PrevLogIndex].CommitTerm {
					reply.RetryIndex = i+1
					break
				}
			}
		}
		log.Printf("Retry term: %d", reply.RetryIndex)

	} else {
		rf.state = Follower
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.config = args.Config
		rf.configNum = args.ConfigNum
		rf.numMissedElections = 0
		reply.Success = true
		rf.persist()

		if len(args.Entries) > 0 {
			log.Printf("entries term: %d", args.Entries[0].CommitTerm)
			rf.log = append(rf.log[:args.PrevLogIndex+1], args.Entries...)
			rf.persist()
		}
		log.Printf("Me: %d | New log length: %d after append entries of length: %d", rf.me, len(rf.log), len(args.Entries))
		// If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
		if args.LeaderCommit > rf.commitIndex {
			if args.LeaderCommit > len(rf.log) - 1 {
				rf.commitIndex = len(rf.log) - 1
			} else {
				rf.commitIndex = args.LeaderCommit
			}
		}
	}
	sendAppendEntriesReply(args.LeaderId, &reply, rf.c)
}
//
// example RequestVote RPC handler.
//

func SendVoteReply(server int, reply *RequestVoteReply, c *websocket.Conn) {
	log.Printf("Sending vote reply")
	temp := &RequestVoteReplyMsg{Action: "onMessage", Audience: rf.peers[server], Type: "RequestVoteReply", Value: reply}
	err := c.WriteJSON(temp)
  if err != nil {
    log.Println("write:", err)
    return
  }
}

func (rf *Raft) RequestVote(args *RequestVoteArgs) {
	log.Printf("Responding to vote request")
	var reply RequestVoteReply
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Me = rf.me
	reply.VoteGranted = false
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm || args.ConfigNum < rf.configNum {
		log.Printf("Me: %d, Received outdated vote request from: %d, our term: %d, their term: %d", rf.me, args.CandidateId, rf.currentTerm, args.Term)
		if (!rf.config[args.CandidateId] && rf.state == Leader) {
			// candidate was not in config and has a lower term
			temp := &Msg{Action: "onMessage", Audience: rf.peers[args.CandidateId], Type: "Kill", Value: []string{""}}
			err := rf.c.WriteJSON(temp)
		  if err != nil {
		    log.Println("write:", err)
		    return
		  }
		} else {
			SendVoteReply(args.Me, &reply, rf.c)
		}
		return
	}
	if (len(rf.log)-1 != -1) {
		lastLogTerm := rf.log[len(rf.log)-1].CommitTerm
		if args.LastLogTerm < lastLogTerm || (args.LastLogTerm == lastLogTerm && args.LastLogIndex < len(rf.log)-1) {
			log.Printf("Me: %d, Received outdated log from vote request from: %d, our log term: %d, their term: %d our index: %d their index: %d", rf.me, args.CandidateId, lastLogTerm, args.LastLogTerm, len(rf.log)-1, args.LastLogIndex)
			if (!rf.config[args.CandidateId] && rf.state == Leader) {
				// candidate was not in config and has an outdated log
				temp := &Msg{Action: "onMessage", Audience: rf.peers[args.CandidateId], Type: "Kill", Value: []string{""}}
				err := rf.c.WriteJSON(temp)
			  if err != nil {
			    log.Println("write:", err)
			    return
			  }
			}
			rf.currentTerm = args.Term
			rf.state = Candidate
			rf.persist()
			SendVoteReply(args.Me, &reply, rf.c)
			return
		}
	}
	if (rf.configNum > args.ConfigNum) {
		log.Printf("Me: %d, Received outdated config from: %d", rf.me, args.CandidateId)
		rf.currentTerm = args.Term
		rf.state = Candidate
		rf.persist()
		SendVoteReply(args.Me, &reply, rf.c)
		return
	}

	if rf.currentTerm < args.Term || rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		log.Printf("Me: %d, Vote granted to request from: %d, our term: %d, their term: %d", rf.me, args.CandidateId, rf.currentTerm, args.Term)
		reply.VoteGranted = true
		rf.numMissedElections = 0
		rf.votedFor = args.CandidateId
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm
		rf.state = Follower
		rf.config = args.Config
		rf.configNum = args.ConfigNum
		rf.persist()
		SendVoteReply(args.Me, &reply, rf.c)
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	// ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	temp := &RequestVoteMsg{Action: "onMessage", Audience: rf.peers[server], Type: "RequestVote", Value: args}
  err := rf.c.WriteJSON(temp)
  if err != nil {
    log.Println("write:", err)
		reply.Term = 0
		reply.VoteGranted = false
		reply.Me = server
    return false
  }
	atomic.StoreInt32(&AwaitingVote, 1)
	log.Printf("Awaiting response...")
	i:=0
	for {
		if atomic.LoadInt32(&AwaitingVote) == 1 {
			time.Sleep(150 * time.Millisecond)
			i++
			log.Printf("%d", i)
			if i == 3 {
				reply.Term = 0
				reply.VoteGranted = false
				reply.Me = server
				log.Printf("Response timed out")
				return false
			}
		} else {
			reply.Term = CurrentVoteReply.Term
			reply.VoteGranted = CurrentVoteReply.VoteGranted
			reply.Me = CurrentVoteReply.Me

			log.Printf("Response actually happened")
			return true
		}
	}
}


func (rf *Raft) requestVotes() {
	// • On conversion to candidate, start election:
	// • Increment currentTerm
	// • Vote for self
	// • Reset election timer
	// • Send RequestVote RPCs to all other servers
	// • If votes received from majority of servers: become leader
	// • If AppendEntries RPC received from new leader: convert to
	// follower
	// • If election timeout elapses: start new election
		rf.mu.Lock()
		if rf.state != Candidate {
			return
		}
		rf.currentTerm += 1
		rf.persist()
		var currTerm = rf.currentTerm
		log.Printf("Me: %d, running election on term: %d", rf.me, currTerm)
		 // ------- part b
		 // ------- part b
		rf.votedFor = rf.me
		votes := 1
		for index, _ := range rf.peers {
			log.Printf("before go func Index of vote req: %d", index)
			if index != rf.me {
				go func(index int, rf *Raft) {
					log.Printf("Index of vote req: %d", index)
					rf.mu.Lock()
					var reply RequestVoteReply
					var args RequestVoteArgs
					if rf.state != Candidate || rf.currentTerm != currTerm {
						rf.mu.Unlock()
						return
					}
					args.Term = currTerm
					args.Me = rf.me
					args.CandidateId = rf.me
					args.LastLogIndex = len(rf.log)-1
					args.LastLogTerm = rf.log[args.LastLogIndex].CommitTerm
					args.Config = rf.config
					args.ConfigNum = rf.configNum
					rf.mu.Unlock()
					ok := rf.sendRequestVote(index, &args, &reply)
					rf.mu.Lock()
					if reply.Term > rf.currentTerm {
						rf.state = Follower
						rf.currentTerm = reply.Term
						rf.persist()
						rf.mu.Unlock()
						return
						// log.Printf("Me: %d is no longer a candidate since larger term is out there", rf.me)
					} else if ok {
						if rf.state != Candidate || rf.currentTerm != currTerm {
							rf.mu.Unlock()
							// log.Printf("Me: %d is no longer a candidate", rf.me)
							return
						}
						if reply.VoteGranted {
							// log.Printf("vote received from index: %d, term: %d", index, args.Term)
							votes += 1
							if votes > rf.N/2 {
								// set state
								if rf.state != Candidate || rf.currentTerm != currTerm {
									rf.mu.Unlock()
									return
								}
								rf.state = Leader
								var nextIndex []int
								var matchIndex []int
								var aliveCheck []int
								i := 0
								for i<len(rf.peers) {
									nextIndex = append(nextIndex, len(rf.log))
									matchIndex = append(matchIndex, 0)
									aliveCheck = append(aliveCheck, 0)

									i++
								}
								rf.nextIndex = nextIndex
								rf.matchIndex = matchIndex
								rf.aliveCheck = aliveCheck
							}
						}
					}
					rf.mu.Unlock()
				}(index, rf)
			}
		}
		rf.mu.Unlock()
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if (rf.state != Leader || rf.dead == 1) {
		return -1, rf.currentTerm, false
	}

	index := len(rf.log)
	rf.matchIndex[rf.me] = index
	term := rf.currentTerm
	isLeader := true
	var entry LogEntry
	entry.CommitTerm = term
	entry.Command = command
	rf.log = append(rf.log, entry)
	rf.persist()
	log.Printf("index to be placed: %d, current term: %d", index, term)
	log.Printf("me: %d | length of log: %d", rf.me, len(rf.log))
	// Your code here (2B).


	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	log.Printf("Killing %d", rf.me)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	// ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	temp := &AppendEntriesArgsMsg{Action: "onMessage", Audience: rf.peers[server], Type: "Heartbeat", Value: args}
  err := rf.c.WriteJSON(temp)
  if err != nil {
    log.Println("write:", err)
		reply.Term = 0
		reply.Success = false
    return false
  }
	atomic.StoreInt32(&AwaitingHB, 1)
	log.Printf("Awaiting HB response...")
	i:=0
	for {
		if atomic.LoadInt32(&AwaitingHB) == 1 {
			time.Sleep(150 * time.Millisecond)
			i++
			log.Printf("%d", i)
			if i == 3 {
				reply.Term = 0
				reply.Success = false
				log.Printf("HB Response timed out")
				return false
			}
		} else {
			reply.Term = CurrentHBReply.Term
			reply.Success = CurrentHBReply.Success
			reply.RetryIndex = CurrentHBReply.RetryIndex
			log.Printf("HB Response actually happened")
			return true
		}
	}
}

func(rf *Raft) requestNewServer(deadServer int) {
	// loop through config to find first available server that responds that isn't this deadServer
	// check if still leader
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader || rf.killed() {
		return
	}
	i := 0
	delete(rf.config, deadServer)
	rf.aliveCheck[deadServer] = 0
	for i < len(rf.peers) {
		if !rf.config[i] {
			temp := &Msg{Action: "onMessage", Audience: rf.peers[i], Type: "AliveCheck", Value: []string{strconv.Itoa(rf.me)}}
			err := rf.c.WriteJSON(temp)
			if err != nil {
				log.Println("write:", err)
			}
		}
	}
}

func (rf *Raft) AddServer(newServer int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	log.Printf("Adding new server: %d", newServer)
	if rf.state != Leader || rf.killed() {
		log.Printf("Just kidding I'm no longer leader or dead")
		return
	}
	if len(rf.config) == rf.N || len(rf.config) <= rf.N/2 {
		log.Printf("No longer needed since config is full or below needed quorum size")
		return
	}
	rf.config[newServer] = true
	rf.configNum++
	// Confirm to new server it's a part of the new config
	temp := &Msg{Action: "onMessage", Audience: rf.peers[newServer], Type: "AddServer", Value: []string{strconv.Itoa(rf.me)}}
	err := rf.c.WriteJSON(temp)
	if err != nil {
		log.Println("write:", err)
	}

}

func(rf *Raft) sendHB() {
	rf.mu.Lock()
	rf.numMissedElections = 0
	if rf.state != Leader {
		return
	}
	log.Printf("%d is the leader of term %d !!!", rf.me, rf.currentTerm)
	logLength := len(rf.log)
	currTerm := rf.currentTerm
	// Lock
	// append entries  (nil for part a)
	for index, peer := range rf.peers {
		if index != rf.me {
			go func(index int, peer *string, rf *Raft) {
				rf.mu.Lock()
				if rf.state != Leader || rf.currentTerm != currTerm {
					rf.mu.Unlock()
					return
				}
				var args AppendEntriesArgs
				args.Term = rf.currentTerm
				args.LeaderId = rf.me
				args.LeaderCommit = rf.commitIndex
				var reply AppendEntriesReply
				args.PrevLogIndex = rf.nextIndex[index]-1
				if args.PrevLogIndex > -1 {
					args.PrevLogTerm = rf.log[args.PrevLogIndex].CommitTerm
				} else {
					args.PrevLogTerm = 0
				}
				args.Entries = rf.log[rf.nextIndex[index]:]
				rf.mu.Unlock()
				ok := sendAppendEntries(index, &args, &reply)
				rf.mu.Lock()
				if rf.state != Leader || rf.currentTerm != currTerm{
					rf.mu.Unlock()
					return
				}
				if ok {
					if reply.Success {
						// log.Printf("Successful | log at index: %d updated to length: %d", index, logLength)
						rf.nextIndex[index] = logLength
						rf.matchIndex[index] = logLength - 1
						rf.aliveCheck[index] = 0
					} else {
						if rf.currentTerm < reply.Term {
							// log.Printf("Leader curr term lower than reply")
							rf.state = Follower
							rf.currentTerm = reply.Term
							rf.persist()
						} else {
							rf.nextIndex[index] = reply.RetryIndex
						}
					}
				} else {
					rf.aliveCheck[index]++
					// handle case when dead
					// Number of missed heartbeats can be changed by 3 to application dependent number
					// Must consider time for new server to come in
					if rf.aliveCheck[index] == 3 {
						i := 0
						numDead := 0
						for i<len(rf.aliveCheck) {
							if rf.aliveCheck[i] >= 2 {
								numDead++
							}
						}
						if numDead <= rf.N/2 {
							log.Printf("Bringing in a new server from the queue")
							go rf.requestNewServer(index)
							// begin bringing new servers in
							// check list
						}
					}
					// alive check is at least 2 to allow a whole round of ensuring others are alive
					// need majority to be alive
					// otherwise you are dead
				}
				rf.mu.Unlock()

			}(index, &peer, rf)
	// set all conditions listed in fig 2
	//check reply to see if reply term > curr term
		}
	}

	rf.mu.Unlock()
	// If there exists an N such that N > commitIndex, a majority
	// of matchIndex[i] ≥ N, and log[N].term == currentTerm:
	// set commitIndex = N (§5.3, §5.4).
	rf.mu.Lock()
	for N := rf.commitIndex + 1; N<len(rf.log); N++ {
		if rf.log[N].CommitTerm > rf.currentTerm {
			log.Printf("Leader had larger commit term than peer")
			break
		} else if rf.log[N].CommitTerm == rf.currentTerm {
			peersConfirmed := 0
			for i := 0; i<len(rf.peers); i++ {
				if rf.matchIndex[i] >= N {
					peersConfirmed ++
				}
			}
			if peersConfirmed > len(rf.peers) / 2 {
				rf.commitIndex = N
			}
		} else {
			log.Printf("Leader commit term was too small so the loop continues")
		}
	}
	rf.mu.Unlock()
}

// func(rf *Raft) checkTimeout(applyCh chan ApplyMsg) {
func(rf *Raft) checkTimeout() {
		go func() {
			for true {
				if rf.killed() {
					return
				}
				rf.mu.Lock()
				if rf.state == Follower {
					rf.state = Candidate
				}
				rf.mu.Unlock()
				time.Sleep(time.Duration(1500 + rand.Intn(250)) * (time.Millisecond))
				var currState State
				rf.mu.Lock()
				if rf.killed() {
					return
				}
				currState = rf.state
				rf.mu.Unlock()
				if currState == Candidate {
					rf.numMissedElections ++
					rf.requestVotes()
				}
				// If 40 elections without a winner has happened, kill this process or create a safe recovery program
				if rf.numMissedElections == 40 {
					rf.Kill()
				}
			}
		}()

		go func() {
			for true {
				if rf.killed() {
					return
				}
				time.Sleep(900 * time.Millisecond)
				var currState State
				rf.mu.Lock()
				if rf.killed() {
					return
				}
				currState = rf.state
				rf.mu.Unlock()
				if currState == Leader {
					rf.sendHB()
				}
			}
		}()


		go func() {
			for true {
				rf.mu.Lock()
				if rf.killed() {
					return
				}
				for rf.commitIndex > rf.lastApplied {
			        rf.lastApplied++
					var message ApplyMsg
					message.CommandValid = true
					message.Command = rf.log[rf.lastApplied].Command
					message.CommandIndex = rf.lastApplied
			    // SEND MESSAGE TO COMMIT SERVER
					log.Printf("Me: %d, Committed at index: %d with term: %d", rf.me, rf.lastApplied, rf.log[rf.lastApplied].CommitTerm)
				}
				rf.mu.Unlock()
				time.Sleep(100*time.Millisecond)
			}
		}()
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(serverMap []string, me int,
	persister *Persister, c *websocket.Conn, N int, config map[int]bool) *Raft {
	rf := &Raft{}
	rf.peers = serverMap
	rf.persister = persister
	rf.me = me
	rf.state = Follower
	rf.currentTerm = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.votedFor = -1
	rf.c = c
	rf.config = config
	rf.configNum = 0
	rf.N = N
	rf.numMissedElections = 0
	atomic.StoreInt32(&AwaitingVote, 0)
	atomic.StoreInt32(&AwaitingHB, 0)
	var nextIndex []int
	var matchIndex []int
	var aliveCheck []int
	i := 0
	for i<len(serverMap) {
		nextIndex = append(nextIndex, 1)
		matchIndex = append(matchIndex, 0)
		aliveCheck = append(aliveCheck, 0)
		i++
	}
	rf.nextIndex = nextIndex
	rf.matchIndex = matchIndex
	rf.aliveCheck = aliveCheck
	// log.Printf("length of nextIndex: %d", len(nextIndex))
	rf.log = []LogEntry{}
	rf.log = append(rf.log, LogEntry{nil, 0})

	// Your initialization code here (2A, 2B, 2C).
	// run requestVoteRPC if hasn't heard from peers in awhile
	// go rf.checkTimeout(applyCh)
	 go rf.checkTimeout()
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())


	return rf
}
