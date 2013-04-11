/*
 * Copyright 2013 Nan Deng
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package msgcenter

import (
	"errors"
	"fmt"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/server"
	"time"
)

type eventConnIn struct {
	errChan chan error
	conn    server.Conn
}

type eventConnLeave struct {
	conn server.Conn
	err  error
}

type EventConnError struct {
	Err error
	C   server.Conn
}

type ServiceConfig struct {
	MaxNrConns        int
	MaxNrUsers        int
	MaxNrConnsPerUser int
}

type ReadMessageError struct {
	conn server.Conn
	err  error
}

func (self *ReadMessageError) Error() string {
	return fmt.Sprintf("%v: %v: %v", self.conn.Service(), self.conn.Username(), self.err)
}

type writeMessageRequest struct {
	user    string
	msg     *proto.Message
	mailbox bool
	timeout time.Duration
	errChan chan<- error
}

type ServiceCenter struct {
	serviceName string
	msgChan     chan<- *proto.Message
	fwdChan     chan<- *server.ForwardRequest
	connErrChan chan<- *EventConnError

	writeReqChan chan *writeMessageRequest
	connIn       chan *eventConnIn
	connLeave    chan *eventConnLeave
}

var ErrTooManyConns = errors.New("too many connections")

func (self *ServiceCenter) process(maxNrConns, maxNrConnsPerUser, maxNrUsers int) {
	connMap := newTreeBasedConnMap()
	nrConns := 0
	for {
		select {
		case connInEvt := <-self.connIn:
			if maxNrConns > 0 && nrConns >= maxNrConns {
				connInEvt.errChan <- ErrTooManyConns
				continue
			}
			err := connMap.AddConn(connInEvt.conn, maxNrConnsPerUser, maxNrUsers)
			if err != nil {
				connInEvt.errChan <- err
				continue
			}
			nrConns++
			connInEvt.errChan <- nil
		case leaveEvt := <-self.connLeave:
			connMap.DelConn(leaveEvt.conn)
			leaveEvt.conn.Close()
			nrConns--
			self.connErrChan <- &EventConnError{C: leaveEvt.conn, Err: leaveEvt.err}
		}
	}
}

func (self *ServiceCenter) serveConn(conn server.Conn) {
	conn.SetForwardRequestChannel(self.fwdChan)
	var err error
	defer func() {
		self.connLeave <- &eventConnLeave{conn: conn, err: err}
	}()
	for {
		var msg *proto.Message
		msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		self.msgChan <- msg
	}
}

func (self *ServiceCenter) NewConn(conn server.Conn) error {
	evt := new(eventConnIn)
	ch := make(chan error)
	evt.conn = conn
	evt.errChan = ch
	self.connIn <- evt
	err := <-ch
	if err == nil {
		go self.serveConn(conn)
	}
	return err
}

func NewServiceCenter(serviceName string, conf *ServiceConfig, msgChan chan<- *proto.Message, fwdChan chan<- *server.ForwardRequest, connErrChan chan<- *EventConnError) *ServiceCenter {
	ret := new(ServiceCenter)
	ret.serviceName = serviceName
	ret.msgChan = msgChan
	ret.connErrChan = connErrChan
	ret.fwdChan = fwdChan

	ret.connIn = make(chan *eventConnIn)
	ret.connLeave = make(chan *eventConnLeave)
	ret.writeReqChan = make(chan *writeMessageRequest)
	go ret.process(conf.MaxNrConns, conf.MaxNrConnsPerUser, conf.MaxNrUsers)
	return ret
}

type writeMessageReq struct {
	srv      string
	usr      string
	msg      *proto.Message
	compress bool
	encrypt  bool
	errChan  chan error
}

var ErrBadMessage = errors.New("malformed message")
var ErrBadService = errors.New("malformed service name")
var ErrBadUsername = errors.New("malformed username")
var ErrNoConn = errors.New("no connection to the specified user")
