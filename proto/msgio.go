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

type ControlCommandProcessor interface {
	ProcessCommand(cmd *Command) (msg *Message, err error)
}

type Conn interface {
	MessageReadWriter
	Close() error
	Service() string
	Username() string
	UniqId() string
}

type messageIO struct {
	conn     net.Conn
	cmdio    *CommandIO
	service  string
	username string
	id       string
	msgChan  chan interface{}
	proc     ControlCommandProcessor
}

func (self *messageIO) Close() error {
	return self.conn.Close()
}

func (self *messageIO) processCommand(cmd *Command) (msg *Message, err error) {
	switch cmd.Type {
	case CMD_BYE:
		err = io.EOF
		return
	}
	if self.proc == nil {
		err = ErrBadPeerImpl
		return
	}
	return self.proc.ProcessCommand(cmd)
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
		if cmd.Type == CMD_DATA {
			if len(cmd.Params) > 0 {
				if cmd.Message == nil {
					cmd.Message = new(Message)
				}
				cmd.Message.Id = cmd.Params[0]
			}
			self.msgChan <- cmd.Message
			continue
		}
		if cmd.Type == CMD_EMPTY {
			if len(cmd.Params) == 0 {
				self.msgChan <- 1
			} else {
				msg := new(Message)
				msg.Id = cmd.Params[0]
				self.msgChan <- msg
			}
			continue
		}

		msg, err := self.processCommand(cmd)
		if err != nil {
			self.msgChan <- err
			return
		}
		if msg != nil {
			self.msgChan <- msg
		}
	}
}

func (self *messageIO) WriteMessage(msg *Message, compress, encrypt bool) error {
	cmd := new(Command)

	if msg != nil {
		cmd.Params = make([]string, 0, 3)
		if msg.IsEmpty() {
			cmd.Type = CMD_EMPTY
		} else if len(msg.Sender) > 0 {
			cmd.Type = CMD_FWD
			cmd.Params = append(cmd.Params, msg.Sender)
			if len(msg.SenderService) > 0 {
				cmd.Params = append(cmd.Params, msg.SenderService)
			}
		} else {
			cmd.Type = CMD_DATA
			cmd.Message = msg
		}
		if len(msg.Id) != 0 {
			cmd.Params = append(cmd.Params, msg.Id)
		}
	} else {
		cmd.Type = CMD_EMPTY
	}
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
	case int:
		msg = nil
		err = nil
	}
	return
}

func NewConn(cmdio *CommandIO, srv, usr string, conn net.Conn, proc ControlCommandProcessor) Conn {
	bufSz := 1024
	ret := new(messageIO)
	ret.conn = conn
	ret.cmdio = cmdio
	ret.service = srv
	ret.username = usr
	ret.msgChan = make(chan interface{}, bufSz)
	ret.proc = proc
	cid, _ := uuid.NewV4()
	ret.id = cid.String()
	go ret.collectMessage()
	return ret
}
