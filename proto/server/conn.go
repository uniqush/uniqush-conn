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
}

type serverConn struct {
	proto.Conn
	cmdio *proto.CommandIO
	digestThreshold int
	compressThreshold int
	digestFields []string
	mcache msgcache.Cache
}

func (self *serverConn) writeDigest(msg *proto.Message, extra map[string]string) (ok bool, err error) {
	ok = false
	if self.digestThreshold < 0 {
		return
	}
	sz := msg.Size()
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

func (self *serverConn) ProcessCommand(cmd *proto.Command) error {
	return nil
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
