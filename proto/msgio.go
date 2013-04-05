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
	"net"
	"io"
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
}

type messageIO struct {
	conn net.Conn
	cmdio *commandIO
	msgChan chan interface{}
}

func (self *messageIO) processCommand(cmd *command) {
}

func (self *messageIO) collectMessage() {
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
		self.processCommand(cmd)
	}
}

func (self *messageIO) WriteMessage(msg *Message, compress, encrypt bool) error {
	cmd := new(command)
	cmd.Type = cmdtype_DATA
	cmd.Message = msg
	return self.cmdio.WriteCommand(cmd, compress, encrypt)
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

func newMessageChannel(cmdio *commandIO, conn net.Conn) Conn {
	bufSz := 1024
	ret := new(messageIO)
	ret.conn = conn
	ret.cmdio = cmdio
	ret.msgChan = make(chan interface{}, bufSz)
	go ret.collectMessage()
	return ret
}


