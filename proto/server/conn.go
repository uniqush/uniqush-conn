/*
 * Copyright 2012 Nan Deng
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

package server

import (
	"fmt"
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/proto"
	"io"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// SendMessage() and ForwardMessage() are goroutine-safe.
// SendMessage() and ForwardMessage() will send a message ditest,
// instead of the message itself, if the message is too large.
// ReceiveMessage() should nevery be called concurrently.
type Conn interface {
	Close() error
	Service() string
	Username() string
	UniqId() string

	// If the message is generated from the server, then use SendMessage()
	// to send it to the client.
	SendMessage(msg *proto.Message, id string, extra map[string]string) error

	// If the message is generated from another client, then
	// use ForwardMessage() to send it to the client.
	ForwardMessage(sender, senderService string, msg *proto.Message, id string) error

	// ReceiveMessage() will keep receiving Commands from the client
	// until it receives a Command with type CMD_DATA.
	ReceiveMessage() (msg *proto.Message, err error)

	SetMessageCache(cache msgcache.Cache)
	Visible() bool
}

type serverConn struct {
	cmdio             *proto.CommandIO
	conn              net.Conn
	compressThreshold int32
	digestThreshold   int32
	service           string
	username          string
	connId            string
	digestFielsLock   sync.Mutex
	digestFields      []string
	cmdProcs          []CommandProcessor
	visible           int32
}

type CommandProcessor interface {
	ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error)
}

func (self *serverConn) Visible() bool {
	v := atomic.LoadInt32(&self.visible)
	return v > 0
}

func (self *serverConn) Close() error {
	return self.conn.Close()
}

func (self *serverConn) Service() string {
	return self.service
}

func (self *serverConn) Username() string {
	return self.username
}

func (self *serverConn) UniqId() string {
	return self.connId
}

func (self *serverConn) shouldCompress(size int) bool {
	t := int(atomic.LoadInt32(&self.compressThreshold))
	if t > 0 && t < size {
		return true
	}
	return false
}

func (self *serverConn) shouldDigest(sz int) bool {
	d := atomic.LoadInt32(&self.digestThreshold)
	if d >= 0 && d < int32(sz) {
		return true
	}
	return false
}

func (self *serverConn) writeDigest(mc *proto.MessageContainer, extra map[string]string, sz int) error {
	digest := &proto.Command{
		Type: proto.CMD_DIGEST,
	}
	params := [4]string{fmt.Sprintf("%v", sz), mc.Id}

	if mc.FromUser() {
		params[2] = mc.Sender
		params[3] = mc.SenderService
		digest.Params = params[:4]
	} else {
		digest.Params = params[:2]
	}

	msg := mc.Message
	header := make(map[string]string, len(extra)+len(msg.Header))
	self.digestFielsLock.Lock()
	defer self.digestFielsLock.Unlock()

	for _, f := range self.digestFields {
		if len(msg.Header) > 0 {
			if v, ok := msg.Header[f]; ok {
				header[f] = v
			}
		}
		if len(extra) > 0 {
			if v, ok := extra[f]; ok {
				header[f] = v
			}
		}
	}
	if len(header) > 0 {
		digest.Message = &proto.Message{
			Header: header,
		}
	}

	compress := self.shouldCompress(digest.Message.Size())
	return self.cmdio.WriteCommand(digest, compress)
}

func (self *serverConn) SendMessage(msg *proto.Message, id string, extra map[string]string) error {
	return self.send(msg, id, extra, true)
}

func (self *serverConn) send(msg *proto.Message, id string, extra map[string]string, tryDigest bool) error {
	if msg == nil {
		cmd := &proto.Command{
			Type: proto.CMD_EMPTY,
		}
		if len(id) > 0 {
			cmd.Params = []string{id}
		}
		return self.cmdio.WriteCommand(cmd, false)
	}
	sz := msg.Size()
	if tryDigest && self.shouldDigest(sz) {
		container := &proto.MessageContainer{
			Id:      id,
			Message: msg,
		}
		return self.writeDigest(container, extra, sz)
	}
	cmd := &proto.Command{
		Type:    proto.CMD_DATA,
		Message: msg,
	}
	cmd.Params = []string{id}
	return self.cmdio.WriteCommand(cmd, self.shouldCompress(sz))
}

func (self *serverConn) ForwardMessage(sender, senderService string, msg *proto.Message, id string) error {
	return self.forward(sender, senderService, msg, id, true)
}

func (self *serverConn) forward(sender, senderService string, msg *proto.Message, id string, tryDigest bool) error {
	sz := msg.Size()
	if sz == 0 {
		return nil
	}
	if tryDigest && self.shouldDigest(sz) {
		container := &proto.MessageContainer{
			Id:            id,
			Sender:        sender,
			SenderService: senderService,
			Message:       msg,
		}
		return self.writeDigest(container, nil, sz)
	}
	cmd := &proto.Command{
		Type:    proto.CMD_FWD,
		Message: msg,
	}
	cmd.Params = []string{sender, senderService, id}
	return self.cmdio.WriteCommand(cmd, self.shouldCompress(sz))
}

func (self *serverConn) processCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	if cmd == nil {
		return
	}

	t := int(cmd.Type)
	if t > len(self.cmdProcs) {
		return
	}
	proc := self.cmdProcs[t]
	if proc != nil {
		msg, err = proc.ProcessCommand(cmd)
	}
	return
}

func (self *serverConn) ReceiveMessage() (msg *proto.Message, err error) {
	var cmd *proto.Command
	for {
		cmd, err = self.cmdio.ReadCommand()
		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				err = io.EOF
			}
			return
		}
		switch cmd.Type {
		case proto.CMD_DATA:
			msg = cmd.Message
			return
		case proto.CMD_BYE:
			err = io.EOF
			return
		default:
			msg, err = self.processCommand(cmd)
			if err != nil || msg != nil {
				return
			}
		}
	}
}

func (self *serverConn) SetMessageCache(cache msgcache.Cache) {
	if cache == nil {
		return
	}
	proc := new(messageRetriever)
	proc.cache = cache
	proc.conn = self
	self.setCommandProcessor(proto.CMD_MSG_RETRIEVE, proc)
}

func (self *serverConn) setCommandProcessor(cmdType uint8, proc CommandProcessor) {
	if cmdType >= proto.CMD_NR_CMDS {
		return
	}
	if len(self.cmdProcs) <= int(cmdType) {
		self.cmdProcs = make([]CommandProcessor, proto.CMD_NR_CMDS)
	}
	self.cmdProcs[cmdType] = proc
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	ret := new(serverConn)
	ret.conn = conn
	ret.cmdio = cmdio
	ret.service = service
	ret.username = username
	ret.connId = fmt.Sprintf("%x-%x", time.Now().UnixNano(), rand.Int63())
	ret.digestThreshold = 1024
	ret.compressThreshold = 1024

	settingproc := new(settingProcessor)
	settingproc.conn = ret
	ret.setCommandProcessor(proto.CMD_SETTING, settingproc)

	visproc := new(visibilityProcessor)
	visproc.conn = ret
	ret.setCommandProcessor(proto.CMD_SET_VISIBILITY, visproc)

	ret.visible = 1
	return ret
}
