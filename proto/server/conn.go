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
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/msgcache"
	"net"
	"fmt"
	"time"
)

type Conn interface {
	// Send the message to client.
	// If the message is larger than the digest threshold,
	// then send a digest to the client and cache the whole message
	// in the message box.
	SendOrBox(msg *proto.Message, extra map[string]string, encrypt bool, timeout time.Duration) error
	SendOrQueue(msg *proto.Message, extra map[string]string, encrypt bool) (id string, err error)
	SetMessageCache(cache msgcache.Cache)
	proto.Conn
}

type serverConn struct {
	proto.Conn
	cmdio *proto.CommandIO
	digestThreshold int
	compressThreshold int
	digestFields []string
	mcache msgcache.Cache
}

func (self *serverConn) writeAutoCompress(msg *proto.Message, encrypt bool, sz int) error {
	compress := false
	if self.compressThreshold > 0 && self.compressThreshold < sz {
		compress = true
	}
	return self.WriteMessage(msg, compress, encrypt)
}

// Send the message to client.
// If the message is larger than the digest threshold,
// then send a digest to the client and cache the whole message
// in the message box.
func (self *serverConn) SendOrBox(msg *proto.Message, extra map[string]string, encrypt bool, timeout time.Duration) error {
	sz := msg.Size()
	sentDigest, err := self.writeDigest(msg, extra, sz, "mbox")
	if err != nil {
		return err
	}

	// We have sent the digest. Cache the message
	if sentDigest {
		err = self.mcache.SetMessageBox(self.Service(), self.Username(), msg, timeout)
		return err
	}

	// Otherwise, send the message directly
	return self.writeAutoCompress(msg, encrypt, sz)
}

func (self *serverConn) SendOrQueue(msg *proto.Message, extra map[string]string, encrypt bool) (id string, err error) {
	sz := msg.Size()
	sentDigest := false
	if self.digestThreshold > 0 && sz > self.digestThreshold {
		sentDigest = true
	}
	// We have sent the digest. Cache the message
	if sentDigest {
		id, err = self.mcache.Enqueue(self.Service(), self.Username(), msg)
		if err != nil {
			return
		}
		if len(id) == 0 || id == "mbox" {
			err = fmt.Errorf("Bad message cache implementation: id=%v", id)
			return
		}
		_, err = self.writeDigest(msg, extra, sz, id)
		if err != nil {
			return
		}
	}
	// Otherwise, send the message directly
	err = self.writeAutoCompress(msg, encrypt, sz)
	id = ""
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

func (self *serverConn) sendMessageInBox() error {
	msg, err := self.mcache.GetMessageBox(self.Service(), self.Username())
	if err != nil {
		return err
	}
	return self.WriteMessage(msg, false, true)
}

func (self *serverConn) ProcessCommand(cmd *proto.Command) error {
	switch cmd.Type {
	case proto.CMD_MSG_RETRIEVE:
		if cmd.Message == nil {
			return proto.ErrBadPeerImpl
		}
	}
	return nil
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
	return sc
}

