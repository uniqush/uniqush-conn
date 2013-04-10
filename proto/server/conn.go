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
	"net"
	"strconv"
	"time"
)

type Conn interface {
	// Send the message to client.
	// If the message is larger than the digest threshold,
	// then send a digest to the client and cache the whole message
	// in the message box.
	SendOrBox(msg *proto.Message, extra map[string]string, timeout time.Duration) error
	SendOrQueue(msg *proto.Message, extra map[string]string) (id string, err error)
	SetMessageCache(cache msgcache.Cache)
	proto.Conn
}

type serverConn struct {
	proto.Conn
	cmdio             *proto.CommandIO
	digestThreshold   int
	compressThreshold int
	encrypt           bool
	digestFields      []string
	mcache            msgcache.Cache
}

func (self *serverConn) shouldDigest(msg *proto.Message) (sz int, sendDigest bool) {
	sz = msg.Size()
	if self.digestThreshold > 0 && self.digestThreshold < sz {
		sendDigest = true
	}
	return
}

func (self *serverConn) writeAutoCompress(msg *proto.Message, sz int) error {
	compress := false
	if self.compressThreshold > 0 && self.compressThreshold < sz && self.mcache != nil {
		compress = true
	}
	return self.WriteMessage(msg, compress, self.encrypt)
}

// Send the message to client.
// If the message is larger than the digest threshold,
// then send a digest to the client and cache the whole message
// in the message box.
func (self *serverConn) SendOrBox(msg *proto.Message, extra map[string]string, timeout time.Duration) error {
	sz, sendDigest := self.shouldDigest(msg)
	if sendDigest {
		err := self.mcache.SetMessageBox(self.Service(), self.Username(), msg, timeout)
		if err != nil {
			return err
		}
	}
	sendDigest, err := self.writeDigest(msg, extra, sz, "mbox")
	if err != nil {
		return err
	}

	// Otherwise, send the message directly
	return self.writeAutoCompress(msg, sz)
}

func (self *serverConn) SendOrQueue(msg *proto.Message, extra map[string]string) (id string, err error) {
	sz, sendDigest := self.shouldDigest(msg)
	if sendDigest {
		id, err = self.mcache.Enqueue(self.Service(), self.Username(), msg)
		if len(id) == 0 || id == "mbox" {
			id = ""
			err = fmt.Errorf("Bad message cache implementation: id=%v", id)
			return
		}
		if err != nil {
			return
		}
	}
	sendDigest, err = self.writeDigest(msg, extra, sz, id)
	if err != nil {
		return
	}

	// Otherwise, send the message directly
	err = self.writeAutoCompress(msg, sz)
	return
}

func (self *serverConn) writeDigest(msg *proto.Message, extra map[string]string, sz int, id string) (sentDigest bool, err error) {

	sentDigest = false
	if self.digestThreshold < 0 {
		return
	}
	if sz < self.digestThreshold {
		return
	}

	digest := new(proto.Command)
	digest.Type = proto.CMD_DIGEST
	digest.Params = make([]string, 2)
	digest.Params[0] = fmt.Sprintf("%v", sz)
	digest.Params[1] = id

	dmsg := new(proto.Message)

	header := make(map[string]string, len(extra))

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
		dmsg.Header = header
		digest.Message = dmsg
	}

	compress := false
	if self.compressThreshold > 0 && self.compressThreshold < sz {
		compress = true
	}

	err = self.cmdio.WriteCommand(digest, compress, true)
	if err != nil {
		return
	}
	sentDigest = true
	return
}

func (self *serverConn) ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	if cmd == nil {
		return
	}
	switch cmd.Type {
	case proto.CMD_SETTING:
		if len(cmd.Params) < 3 {
			err = proto.ErrBadPeerImpl
			return
		}
		if len(cmd.Params[0]) > 0 {
			self.digestThreshold, err = strconv.Atoi(cmd.Params[0])
			if err != nil {
				err = proto.ErrBadPeerImpl
				return
			}
		}
		if len(cmd.Params[1]) > 0 {
			self.compressThreshold, err = strconv.Atoi(cmd.Params[1])
			if err != nil {
				err = proto.ErrBadPeerImpl
				return
			}
		}
		if len(cmd.Params[2]) > 0 {
			if cmd.Params[2] == "0" {
				self.encrypt = false
			} else if cmd.Params[2] == "1" {
				self.encrypt = true
			}
		}
		if len(cmd.Params) > 3 {
			self.digestFields = make([]string, len(cmd.Params)-3)
			for i, f := range cmd.Params[3:] {
				self.digestFields[i] = f
			}
		}
	case proto.CMD_MSG_RETRIEVE:
		if len(cmd.Params) < 1 {
			err = proto.ErrBadPeerImpl
			return
		}
		id := cmd.Params[0]

		// If there is no cache, then send an empty message
		if self.mcache == nil {
			m := new(proto.Message)
			m.Id = id
			err = self.writeAutoCompress(m, m.Size())
			return
		}

		var rmsg *proto.Message

		switch id {
		case "mbox":
			rmsg, err = self.mcache.GetMessageBox(self.Service(), self.Username())
		default:
			rmsg, err = self.mcache.DelFromQueue(self.Service(), self.Username(), id)
		}
		if err != nil {
			return
		}

		if rmsg == nil {
			rmsg = new(proto.Message)
		}
		rmsg.Id = id
		err = self.writeAutoCompress(rmsg, rmsg.Size())
	}
	return
}

func (self *serverConn) SetMessageCache(cache msgcache.Cache) {
	self.mcache = cache
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	sc := new(serverConn)
	sc.cmdio = cmdio
	c := proto.NewConn(cmdio, service, username, conn, sc)
	sc.Conn = c
	sc.digestThreshold = -1
	sc.compressThreshold = 512
	sc.digestFields = make([]string, 0, 10)
	sc.encrypt = true
	return sc
}
