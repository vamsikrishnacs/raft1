raft1
=====

raft leader election algorithm
-----------------------

About:
---------
Implemeted Raft leader election algorithm in go language.It is essentially a group of servers communicating via RPC's to elect a leader using the leader elction algorithm in RAFT.

Features:
-----------
1. The package raft is a replicator object capable of communicating with other raft objects in order to establish a leader.
2. A New object can be created using

raft.New(1,config file path)

3. The replicators communicate using the zeromq message passing mechanism and conisits of inbox() and outbox().In order to send a message

s.outbox()<-&raft.Envelope{pid,Message}

4. The internal communication is automatically handled within the raft object itself and it is capable of finding the leader or establish itself as a leader.

5.Provide a configuration file which contains peer adresses,election-timeouts,default heartbeat-interval


Usage:
----------------
1. go get github.com/vamsikrishnacs/raft.go
2. go test

Output:
---------------
1. A term.txt file updated with the term leaders (leader,termid).the file is updated until no leader is formed i.e none have the majority to become a leader(Usually happens after killing the half of the servers)

Testing:
-------------------
1. Used a timeout to kill the leader every 7 seconds,after a leader is killed candidates emerge and after necessary processing(election) a new leader emerge.
2. After killing half of the processes no update to file is made i.e..,system comes to a halt
3. while testing a (tick) in command line is seen to ensure the Heartbeat(a leader) is live.
4. At halt state (tick) stops








