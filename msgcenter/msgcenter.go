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
	"github.com/uniqush/uniqush-conn/proto"
	"net"
	"crypto/rsa"
	"sync/atomic"
	"time"
	"strings"
	"fmt"
)

type MessageContainer struct {
	Service string
	User string
	Message *proto.Message
}

type ControlMessage struct {
	Service string
	User string
	Command int
	Params [][]byte
}

type MessageCenter struct {
	ln net.Listener
	priv *rsa.PrivateKey
	auth proto.Authenticator
	authTimeout time.Duration
	maxNrConns int32

	msgInQueue chan<- *MessageContainer
	ctrlQueue chan<- *ControlMessage
	errChan chan<- error

	nrConns int32
	msgOutQueue chan *writeMessageReq
	connInQueue chan proto.Conn
	connOutQueue chan proto.Conn
}

func NewMessageCenter(ln net.Listener,
	privkey *rsa.PrivateKey,
	auth proto.Authenticator,
	authTimeout time.Duration,
	maxNrConn int,
	msgChan chan<- *MessageContainer,
	ctrlChan chan<- *ControlMessage,
	errChan chan<- error) *MessageCenter {

	ret := new(MessageCenter)
	ret.ln = ln
	ret.priv = privkey
	ret.auth = auth
	ret.authTimeout = authTimeout
	ret.maxNrConn = maxNrConn
	ret.msgInQueue = msgChan
	ret.ctrlQueue = ctrlChan
	ret.errChan = errChan
	go ret.receiveConnections()
	go ret.messageWriter()
	return ret
}

type writeMessageReq struct {
	srv string
	usr string
	msg *proto.Message
	compress bool
	encrypt bool
	errChan chan<- error
}

var ErrBadMessage = errors.New("malformed message")
var ErrBadService = errors.New("malformed service name")
var ErrBadUsername = errors.New("malformed username")
var ErrNoConn = errors.New("no connection to the specified user")

func (self *MessageCenter) SendMessage(service, username string, msg *proto.Message, compress, encrypt bool) error {
	if msg == nil {
		return ErrBadMessage
	}
	if strings.Contains(service, "\n") {
		return ErrBadService
	}
	if strings.Contains(username, "\n") {
		return ErrBadUsername
	}
	req := new(writeMessageReq)
	req.srv = service
	req.usr = username
	req.msg = msg
	req.errChan = make(chan error)
	req.compress = compress
	req.encrypt = encrypt
	self.msgOutQueue <- req
	err := <-req.errChan
	if err != nil {
		return err
	}
	return nil
}

func (self *MessageCenter) messageWriter() {
	connMap := newTreeBasedConnMap()
	for {
		select {
		case conn := <-self.connInQueue:
			connMap.AddConn(conn)
		case conn := <-self.connOutQueue:
			connMap.DelConn(conn)
		case req := <-self.msgOutQueue:
			conns := connMap.GetConn(req.srv, req.usr)
			if len(conns) == 0 {
				req.errChan <- ErrNoConn
				continue
			}
			go func() {
				for _, c := range conns {
					err := c.WriteMessage(req.msg, req.compress, req.encrypt)
					if err != nil {
						req.errChan <- err
						break
					}
				}
				close(req.errChan)
			}()
		}
	}
}

func (self *MessageCenter) serveClient(c net.Conn) {
	defer c.Close()
	defer atomic.AddInt32(&self.nrConns, -1)
	conn, err := AuthConn(c, self.priv, self.auth, self.timeout
	if err != nil {
		if self.errChan != nil {
			self.errChan <- fmt.Errorf("Connection from %v failed: %v", c.RemoteAddr(), err)
		}
		return
	}
	username := conn.Username()
	service := conn.Service()
	self.connInQueue <- conn
	defer func() {
		self.connOutQueue <- conn
	}()

	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			if self.errChan != nil {
				self.errChan <- fmt.Errorf("[Service=%v][User=%v][Addr=%v] Read failed: %v", service, username, c.RemoteAddr(), err)
			}
			return
		}
		// XXX A memory pool? Maybe.
		m := new(MessageContainer)
		m.Service = service
		m.User = username
		m.Message = msg
		self.msgInQueue <- m
	}
}

func (self *MessageCenter) receiveConnections() {
	for {
		conn, err := self.ln.Accept()
		if err != nil {
			self.erroChan <- err
			return
		}
		if self.maxNrConns > 0 && atomic.LoadInt32(&self.nrConns) >= self.maxNrConns {
			self.erroChan <- fmt.Errorf("[Addr=%v] Refuse the connection: exceed maximum number of connections", conn.RemoteAddr())
			conn.Close()
			continue
		}
		atomic.AddInt32(&self.nrConns, 1)
		go self.serveClient(conn)
	}
}

