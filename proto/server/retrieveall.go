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
	"github.com/uniqush/uniqush-conn/msgcache"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/rpc"
	"strconv"
	"time"
)

type retriaveAllMessages struct {
	conn  *serverConn
	cache msgcache.Cache
}

func cutString(data []byte) (str, rest []byte, err error) {
	var idx int
	var d byte
	idx = -1
	for idx, d = range data {
		if d == 0 {
			break
		}
	}
	if idx < 0 {
		err = proto.ErrMalformedCommand
		return
	}
	str = data[:idx]
	rest = data[idx+1:]
	return
}

func (self *retriaveAllMessages) sendAllCachedMessage(since time.Time) error {
	mcs, err := self.cache.RetrieveAllSince(self.conn.Service(), self.conn.Username(), since)
	if err != nil {
		return err
	}
	if len(mcs) == 0 {
		return nil
	}
	for _, mc := range mcs {
		if mc == nil {
			continue
		}
		if mc.FromServer() {
			err = self.conn.SendMessage(mc.Message, mc.Id, nil, true)
		} else {
			err = self.conn.ForwardMessage(mc.Sender, mc.SenderService, mc.Message, mc.Id, true)
		}
	}
	return nil
}

func (self *retriaveAllMessages) ProcessCommand(cmd *proto.Command) (msg *rpc.Message, err error) {
	if cmd == nil || cmd.Type != proto.CMD_REQ_ALL_CACHED || self.conn == nil || self.cache == nil {
		return
	}
	since := time.Time{}
	if len(cmd.Params) > 0 {
		unix, err := strconv.ParseInt(cmd.Params[0], 10, 64)
		if err != nil {
			return nil, proto.ErrBadPeerImpl
		}
		since = time.Unix(unix, 0)
	}
	err = self.sendAllCachedMessage(since)
	return
}
