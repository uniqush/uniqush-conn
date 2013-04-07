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

package proto

import (
	"github.com/nu7hatch/gouuid"
	"io"
	"net"
	"sync/atomic"
)

type MessageWriter interface {
	WriteMessage(msg *Message, compress, encrypt bool) error
}

type MessageReader interface {
	ReadMessage() (msg *Message, err error)
}

type MessageReadWriter interface {
	MessageReader
	MessageWriter
}

type Conn interface {
	MessageReadWriter
	Service() string
	Username() string
	UniqId() string

	// If a connection is visible, then the device(s) under the service/user
	// will not receive any push notification.
	// Otherwise, the message will be still delivered through this connection,
	// but the notifications will be sent as well.
	Invisible() bool
}

type messageIO struct {
	conn     net.Conn
	cmdio    *commandIO
	service  string
	username string
	id       string
	invisible int32
	msgChan  chan interface{}
}

func (self *messageIO) Invisible() bool {
	return atomic.LoadInt32(&self.invisible) == 1
}

func (self *messageIO) processCommand(cmd *command) error {
	switch cmd.Type {
	case cmdtype_BYE:
		return io.EOF
	case cmdtype_INVIS:
		atomic.StoreInt32(&self.invisible, 1)
	case cmdtype_VIS:
		atomic.StoreInt32(&self.invisible, 0)
	}
	return nil
}

func (self *messageIO) collectMessage() {
	defer self.conn.Close()
	for {
		cmd, err := self.cmdio.ReadCommand()
		if err != nil {
			if err == io.EOF {
				self.msgChan <- io.EOF
				// Closed channel
				return
			}
			self.msgChan <- err
			continue
		}
		if cmd == nil {
			continue
		}
		if cmd.Type == cmdtype_DATA {
			self.msgChan <- cmd.Message
			continue
		}
		err = self.processCommand(cmd)
		if err != nil {
			self.msgChan <- err
			return
		}
	}
}

func (self *messageIO) WriteMessage(msg *Message, compress, encrypt bool) error {
	cmd := new(command)
	cmd.Type = cmdtype_DATA
	cmd.Message = msg
	return self.cmdio.WriteCommand(cmd, compress, encrypt)
}

func (self *messageIO) UniqId() string {
	return self.id
}

func (self *messageIO) Service() string {
	return self.service
}

func (self *messageIO) Username() string {
	return self.username
}

func (self *messageIO) ReadMessage() (msg *Message, err error) {
	d := <-self.msgChan
	switch t := d.(type) {
	case *Message:
		msg = t
	case error:
		err = t
	}
	return
}

func newMessageChannel(cmdio *commandIO, srv, usr string, conn net.Conn) Conn {
	bufSz := 1024
	ret := new(messageIO)
	ret.conn = conn
	ret.cmdio = cmdio
	ret.service = srv
	ret.username = usr
	ret.msgChan = make(chan interface{}, bufSz)
	cid, _ := uuid.NewV4()
	ret.id = cid.String()
	go ret.collectMessage()
	return ret
}
