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
)

type messageRetriever struct {
	conn  *serverConn
	cache msgcache.Cache
}

func (self *messageRetriever) ProcessCommand(cmd *proto.Command) (msg *rpc.Message, err error) {
	if cmd == nil || cmd.Type != proto.CMD_MSG_RETRIEVE || self.conn == nil || self.cache == nil {
		return
	}
	if len(cmd.Params) < 1 {
		err = proto.ErrBadPeerImpl
		return
	}
	id := cmd.Params[0]
	mc, err := self.cache.Get(self.conn.Service(), self.conn.Username(), id)
	if err != nil {
		return
	}
	if mc == nil || mc.Message == nil {
		err = self.conn.send(nil, id, nil, false)
		return
	}
	if mc.FromServer() {
		err = self.conn.send(mc.Message, id, nil, false)
	} else {
		err = self.conn.forward(mc.Sender, mc.SenderService, mc.Message, id, false)
	}
	return
}
