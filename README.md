# DistSysProject
Final Project of Distributed Systems

## How to run locally
- Each Raft folder contains a server that can run Raft
- cd into main folder and run `go run .` to initiate server
- Each server should be run in a different terminal window and use a different folder (otherwise unique id of server will be the same)

### How to set message or kill a server
- cd into RaftClient and run `go run .`
- Calls and timer can be adjusted within the file