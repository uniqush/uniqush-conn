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
)

type Conn interface {
	proto.Conn
	// Send the message to client.
	// If the message is larger than the digest threshold,
	// then send a digest to the client and cache the whole message
	// in the message box.
	SendOrBox(msg *proto.Message, extra map[string]string encrypt bool) error
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
func (self *serverConn) SendOrBox(msg *proto.Message, extra map[string]string, encrypt bool) error {
	sz := msg.Size()
	ok, err := self.writeDigest(msg, extra, sz)
	if err != nil {
		return err
	}

	// We have sent the digest. Cache the message
	if ok {
		err = self.mcache.SetMessageBox(self.Service(), self.Username(), msg)
		return err
	}
	return self.writeAutoCompress(msg, encrypt, sz)
}

func (self *serverConn) writeDigest(msg *proto.Message, extra map[string]string, sz int) (ok bool, err error) {
	ok = false
	if self.digestThreshold < 0 {
		return
	}
	if sz < self.digestThreshold {
		return
	}

	digest := new(proto.Command)
	digest.Type = proto.CMD_DIGEST
	digest.Params = make([]string, 1)
	digest.Params[0] = fmt.Sprintf("%v", sz)

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
	ok = true
	return
}

func (self *serverConn) sendMessageInBox() error {
}

func (self *serverConn) ProcessCommand(cmd *proto.Command) error {
	return nil
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn, cache msgcache.Cache) Conn {
	sc := new(serverConn)
	sc.cmdio = cmdio
	c := proto.NewConn(cmdio, service, username, conn, sc)
	sc.Conn = c
	sc.digestThreshold = -1
	sc.compressThreshold = 512
	sc.digestFields = make([]string, 0, 10)
	sc.mcache = cache
	return sc
}
