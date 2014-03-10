package raft

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

const (
	BROADCAST = -1
)

type Envelope struct {
	// On the sender side, Pid identifies the receiving peer. If instead, Pid is
	// set to cluster.BROADCAST, the message is sent to all peers. On the receiver side, the
	// Id is always set to the original sender. If the Id is not found, the message is silently dropped
	Pid int

	// An id that globally and uniquely identifies the message, meant for duplicate detection at
	// higher levels. It is opaque to this package.
	MsgId int64

	// the actual message.
	Msg interface{}
}

/*type Server interface {
    // Id of this server
    Pid() int

    // array of other servers' ids in the same cluster
    Peers() []int


    // the channel to use to send messages to other peers
    // Note that there are no guarantees of message delivery, and messages
    // are silently dropped
    Outbox() chan *Envelope


    // the channel to receive messages from other peers.
    Inbox() chan *Envelope
}
*/

const (
	S = "stopped"
	F = "follower"
	C = "candidate"
	L = "leader"
)

type Server struct {
	serverID      int
	serverAddress string
	peers         []int
	peerAddress   map[string]string
	outbox        chan *Envelope
	inbox         chan *Envelope
	c1            int
	//persistent state
	votedFor    int
	currentTerm uint64
	state       string
	/*
	        //commitindex uint64
		log *Log
		leader int
		stopped chan bool*/
	c chan *ev

	//timeout	
	Electiontimeout   time.Duration
	Heartbeatinterval time.Duration
}

// Id of this server

type ev struct {
	req interface{}
	t   string
}

type Appendentryreq struct {
	Term     uint64
	Leaderid int
}

type Requestvotereq struct {
	Term        uint64
	Candidateid int
}

type response struct {
	Updateterm uint64
	Success    bool
}

func init() {
	gob.Register(Appendentryreq{})
	gob.Register(Requestvotereq{}) // give it a dummy VoteRequestObject.
	gob.Register(response{})
}

type cc struct {
	Ser               map[string]string
	Send              int
	Receive           int
	Peer              []int
	Electiontimeout   []time.Duration
	Heartbeatinterval time.Duration
}

//Creating the cluster

func New(id int, path string, ch chan bool) Server {

	fmt.Println("INITIALISING SERVER")
	fmt.Println(id)
	//1.reading the configuration file
	file, e := ioutil.ReadFile(path)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}

	var c cc
	json.Unmarshal(file, &c)
	fmt.Println(c)
	id1 := strconv.Itoa(id)
	//2.initialising

	o := make(chan *Envelope, 1000)
	i := make(chan *Envelope, 1000)

	s := Server{id, c.Ser[id1], c.Peer, c.Ser, o, i, 0, 0, 0, F, make(chan *ev), c.Electiontimeout[id-1], c.Heartbeatinterval}

	//3.handling server sockets
	go handleServerrep(&s)
	go handleServerreq(&s)
	go s.loop(ch)

	return s
}

func (s *Server) stop(respChan chan bool, ch chan bool) {
	select {
	case <-ch:
		s.stopheartbeat(respChan)
		s.state = S
		t1, _ := os.OpenFile("term.txt", os.O_APPEND|os.O_WRONLY, 0600)
		t1.WriteString(" ##stopped## ")
	}

}

func (s *Server) loop(ch chan bool) {

	for s.state != S {
		st := s.state
		fmt.Println("started")

		//s.debugln("server.loop.run ", state)
		switch st {
		case F:
			s.followerLoop()
		case C:
			s.candidateLoop()
		case L:
			s.leaderLoop(ch)
		case S:
			return
		}
	}

}

func (s *Server) startheartbeat(stop chan bool) {
	ticker := time.NewTicker(s.HeartbeatInterval())
	for {
		select {

		case <-stop:
			return

		case <-ticker.C:
			fmt.Println("tick")
			req := Appendentryreq{s.currentTerm, s.serverID}
			s.sendAppendEntry(req)

		}
	}

}

func (s *Server) stopheartbeat(stop chan bool) {
	stop <- true
}

//controller
func (s *Server) leaderLoop(ch chan bool) {

	//write term to disc  

	t1, _ := os.OpenFile("term.txt", os.O_APPEND|os.O_WRONLY, 0600)
	t1.WriteString(" ##leader## ")
	sid := strconv.Itoa(s.serverID)
	tid := strconv.Itoa(int(s.currentTerm))
	t1.WriteString(" (" + sid + ",")
	t1.WriteString(tid + ")")
	//startheartbeat
	respChan := make(chan bool)
	go s.startheartbeat(respChan)

	for s.state == L {

		sid := strconv.Itoa(s.serverID)
		tid := strconv.Itoa(int(s.currentTerm))
		t1.WriteString(" (" + sid + ",")
		t1.WriteString(tid + ")")

		//s.state=S
		go s.stop(respChan, ch)

		//startheartbeat() 

		select {

		//election timeout
		case <-time.After(5 * time.Second):
			s.currentTerm++
			//s.stopheartbeat(respChan)
			//s.state=S

		}
	}
}

func (s *Server) followerLoop() {

	for s.state == F {

		select {
		//process RPC's
		case request := <-s.Inbox():

			switch req := request.Msg.(type) {

			case Appendentryreq:
				resp := s.pAppendentry(req)
				s.sendAppendEntryresponse(request.Pid, resp)
			case Requestvotereq:
				resp := s.pRequestvote(req)
				s.sendRequestVoteresponse(request.Pid, resp)
			default:

			}
			//election timeout
		case <-time.After(s.ElectionTimeout()):

			s.state = C
		}
	}
}

func (s *Server) candidateLoop() {

	dovote := true
	var votes int
	for s.state == C {
		if dovote {
			votes = 0
			// Increment current term, vote for self.
			s.currentTerm++

			s.votedFor = s.serverID
			votes = votes + 1

			req := Requestvotereq{s.currentTerm, s.serverID}

			s.sendRequestVote(req)

			dovote = false
		}
		select {

		//process RPC's
		case request := <-s.Inbox():

			switch req := request.Msg.(type) {

			case response:
				t1, _ := os.OpenFile("term.txt", os.O_APPEND|os.O_WRONLY, 0600)

				sid := strconv.Itoa(s.serverID)
				tid := strconv.Itoa(votes)
				t1.WriteString(" *" + sid + ",")
				t1.WriteString(tid + "*")
				if req.Success == true {

					votes++

					if votes >= 4 /*(len(s.peers)/2)*/ {

						s.state = L
					}
				} else {

					//s.state=F

				}

			case Appendentryreq:
				resp := s.pAppendentry(req)
				//processed entry results in state change C to F	
				s.sendAppendEntryresponse(request.Pid, resp)

			case Requestvotereq:
				resp := s.pRequestvote(req)
				s.sendRequestVoteresponse(request.Pid, resp)

			}

			//electiontimeout
		case <-time.After(s.ElectionTimeout()):

			dovote = true
		}

	}

}

//rpc replies

func (s *Server) sendAppendEntryresponse(pid int, resp response) {

	s.outbox <- &Envelope{Pid: pid, Msg: resp}
}

func (s *Server) sendRequestVoteresponse(pid int, resp response) {

	s.Outbox() <- &Envelope{Pid: pid, Msg: resp}
}

//rpc's send

func (s *Server) sendRequestVote(req Requestvotereq) {

	s.Outbox() <- &Envelope{Pid: -1, Msg: req}

}

func (s *Server) sendAppendEntry(req Appendentryreq) {

	s.outbox <- &Envelope{Pid: -1, Msg: req}
}

//dtect terms from req
//receivers implementation of requestvote  state=L/F 
func (s *Server) pRequestvote(req Requestvotereq) response {
	var s1 response

	//receiver
	if req.Term < s.currentTerm {
		s1 = response{s.currentTerm, false}

	} else {
		s.currentTerm = req.Term
		s.state = F
		if s.votedFor == 0 || s.votedFor == req.Candidateid {
			s.votedFor = req.Candidateid
			s1 = response{s.currentTerm, true}
		} else {
			s1 = response{s.currentTerm, false}
		}
	}

	return s1
}

//receivers implementation of Appendentry state=C/F

func (s *Server) pAppendentry(req Appendentryreq) response { //reqchan
	var s1 response
	//receiver
	if req.Term < s.currentTerm {
		s1 = response{s.currentTerm, false}
	} else {
		s.votedFor = 0
		s.currentTerm = req.Term
		s.state = F
		s1 = response{s.currentTerm, true}
	}
	return s1
}

//Cluster code--------
//---------Handles sending and receiving of RPCs

func (serv *Server) Pid() int {
	return serv.serverID
}

func (serv *Server) ElectionTimeout() time.Duration {
	return serv.Electiontimeout

}

func (serv *Server) HeartbeatInterval() time.Duration {
	return serv.Heartbeatinterval
}

// array of other servers' ids in the same cluster
func (serv *Server) Peers() []int {
	return serv.peers
}

// the channel to use to send messages to other peers
// Note that there are no guarantees of message delivery, and messages are silently dropped
func (serv *Server) Outbox() chan *Envelope {
	return serv.outbox
}

// the channel to receive messages from other peers.
func (s *Server) Inbox() chan *Envelope {
	return s.inbox
}

//SUBROUTINE RECEIVE

func handleServerrep(s *Server) {

	//1.point-point messaging

	socketrep, _ := zmq.NewSocket(zmq.REP)
	/*
	   err := socket.Connect(PROTOCOL + addr)

	*/
	socketrep.Bind("tcp://" + s.serverAddress)

	//socket.Bind("tcp://127.0.0.1:6000")
	s.c1 = 0
	for {
		//decoding
		msg, err := socketrep.RecvBytes(0)
		if err != nil {
			fmt.Println(err)
		}
		socketrep.Send("Ack ok", 0)
		var msg1 Envelope
		pCache := bytes.NewBuffer(msg)
		decCache := gob.NewDecoder(pCache)
		decCache.Decode(&msg1)

		s.inbox <- &msg1 //&Envelope{1,1,msg}
		s.c1 = s.c1 + 1

	}

}

//SUBROUTINE SEND

func handleServerreq(s *Server /*,pid int*/) {

	//send msgs 
	for {
		select {
		case envelope := <-s.Outbox():

			if envelope.Pid == -1 {

				Sendbroadcastmessage(envelope, s)
			} else {

				Sendmessage(envelope, s)
			}
		case <-time.After(2 * time.Second):
			fmt.Println("Waited and waited. Ab thak gaya\n")
		}
	}
}

func Sendmessage(envlope *Envelope, s *Server) {

	peerid := (envlope.Pid)
	//1.point-point messaging
	socketreq, _ := zmq.NewSocket(zmq.REQ)
	t := strconv.Itoa(peerid)
	w := strconv.Itoa(s.serverID)
	fmt.Println("Connectresp from=---" + w)
	socketreq.Connect("tcp://" + s.peerAddress[t])

	envlope.Pid = s.serverID

	//ms,_:=json.Marshal(*envlope)
	encodeData := envlope
	mCache := new(bytes.Buffer)
	encCache := gob.NewEncoder(mCache)
	encCache.Encode(encodeData)

	//socketreq.SendBytes(ms, 0)
	socketreq.SendBytes(mCache.Bytes(), 0)

	_, err := socketreq.Recv(0)
	if err != nil {
		fmt.Println(err)
	}
	//s.inbox<-&Envelope{int(s.serverAddress),2,m}

}

func Sendbroadcastmessage(envlope *Envelope, s *Server) {

	//1.broad-cast messaging
	socketreq, _ := zmq.NewSocket(zmq.REQ)
	envlope.Pid = s.serverID

	//ms,_:=json.Marshal(Envelope{1,1,})

	//Encode
	encodeData := envlope
	mCache := new(bytes.Buffer)
	encCache := gob.NewEncoder(mCache)
	encCache.Encode(encodeData)
	//fmt.Println(mCache.Bytes())

	for j1 := range s.peers {
		j := strconv.Itoa(j1 + 1)
		//fmt.Println(j)
		if (j1 + 1) != s.serverID {

			socketreq.Connect("tcp://" + s.peerAddress[j])

		}
	}

	for j1 := range s.peers {
		if (j1 + 1) != s.serverID {
			//fmt.Println("send actual")
			//fmt.Println(ms)		
			socketreq.SendBytes(mCache.Bytes(), 0)
			_, err := socketreq.Recv(0)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	//s.inbox<-&Envelope{int(s.serverAddress),2,m}
}
