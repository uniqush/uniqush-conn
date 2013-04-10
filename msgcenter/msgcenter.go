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
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/server"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

type eventConnIn struct {
	errChan chan error
	conn server.Conn
}

type MessageCenter struct {
	serviceName string
	msgChan chan<- *proto.Message
	errChan chan<- error
	fwdChan chan<- *server.ForwardRequest

	connIn chan *eventConnIn
	connLeave chan server.Conn
}

var ErrTooManyConns = errors.New("too many connections")

func (self *MessageCenter) process(maxNrConns, maxNrConnsPerUser, maxNrUsers int) {
	connMap := newTreeBasedConnMap()
	nrConns := 0
	for {
		select {
		case connInEvt := <-self.connIn:
			if maxNrConns > 0 && nrConns >= maxNrConns {
				connInEvt.errChan <- ErrTooManyConns
				continue
			}
			err := connMap.AddConn(connInEvt, maxNrConnsPerUser, maxNrUsers)
			if err != nil {
				connInEvt.errChan <- err
				continue
			}
			nrConns++
			connInEvt.errChan <- nil
		}
	}
}

func (self *MessageCenter) serveConn(conn server.Conn) {
	conn.SetForwardRequestChannel(self.fwdChan)
	for {
		msg, err := conn.ReadMessage()
		if msg == nil {
			msg = new(proto.Message)
		}
	}
}

func (self *MessageCenter) NewConn(conn server.Conn) error {
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

func NewMessageCenter(serviceName string, maxNrConns, maxNrConnsPerUser, maxNrUsers int, msgChan chan<- *proto.Message, errChan chan<- error) *MessageCenter {
	ret := new(MessageCenter)
	ret.serviceName = serviceName
	ret.maxNrConns = maxNrConns
	ret.msgChan = msgChan

	ret.connIn = make(chan *eventConnIn)
	ret.connLeave = make(chan server.Conn)
	ret.errChan = errChan
	go self.process(maxNrConns, maxNrConnsPerUser, maxNrUsers)
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
