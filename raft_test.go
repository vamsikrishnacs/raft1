package raft

import (
	"fmt"
	"testing"
	"time"
)

//import "log"

func Test(t *testing.T) {

	ch := make(chan bool)
	//ch1:=make(chan bool)
	New(1, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	New(2, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	s3 := New(3, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	New(4, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	New(5, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	New(6, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	New(7, "/home/vamsi/Desktop/go/src/github.com/vamsikrishnacs/cluster1/d.json", ch)
	//s3.Outbox()<-&Envelope{2,0,"hii"}
	//s4.Outbox()<-&Envelope{2,0,"hiiii"}

	for {
		select {

		case <-time.After(7 * time.Second):

			ch <- true

		}
	}
	for {

		select {
		case <-s3.Inbox():
			fmt.Println("inbox")
			//case q:=<-s2.Inbox():
			//fmt.Println("s2---------------------------------------------------------------*********1")
			//fmt.Println(q.Msg)
		}

	}

}
