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
	"fmt"
)

type MessageContainer struct {
	Service string
	User string
	Message *proto.Message
}

type MessageCenter struct {
	ln net.Listener
	msgInQueue chan *MessageContainer
	msgOutQueue chan *MessageContainer
	connInQueue chan proto.Conn
	maxNrConns int32
	nrConns int32

	errChan chan error
	priv *rsa.PrivateKey
	auth proto.Authenticator
	authTimeout time.Duration
}

func (self *MessageCenter) messageWriter() {
	connMap := make(map[string]proto.Conn, 1024)
	for {
	select {
	case conn := <-self.connInQueue:

	case msgCtn := <-self.msgOutQueue:
	}
}

func (self *MessageCenter) serveClient(c net.Conn) {
	defer c.Close()
	defer atomic.AddInt32(&self.nrConns, -1)
	conn, err := AuthConn(c, self.priv, self.auth, self.timeout
	if err != nil {
		self.errChan <- fmt.Errorf("Connection from %v failed: %v", c.RemoteAddr(), err)
		return
	}
	username := conn.Username()
	service := conn.Service()
	self.connInQueue <- conn

	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			self.errChan <- fmt.Errorf("[Service=%v][User=%v][Addr=%v] Read failed: %v", service, username, c.RemoteAddr(), err)
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
		}
		atomic.AddInt32(&self.nrConns, 1)
		go self.serveClient(conn)
	}
}

