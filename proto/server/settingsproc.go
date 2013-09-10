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
	"github.com/uniqush/uniqush-conn/rpc"
	"strconv"
	"sync/atomic"
)

type settingProcessor struct {
	conn *serverConn
}

func (self *settingProcessor) ProcessCommand(cmd *proto.Command) (msg *rpc.Message, err error) {
	if cmd.Type != proto.CMD_SETTING || self.conn == nil {
		return
	}
	if len(cmd.Params) < 2 {
		err = proto.ErrBadPeerImpl
		return
	}
	if len(cmd.Params[0]) > 0 {
		var d int
		d, err = strconv.Atoi(cmd.Params[0])
		if err != nil {
			err = proto.ErrBadPeerImpl
			return
		}
		atomic.StoreInt32(&self.conn.digestThreshold, int32(d))

	}
	if len(cmd.Params[1]) > 0 {
		var c int
		c, err = strconv.Atoi(cmd.Params[1])
		if err != nil {
			err = proto.ErrBadPeerImpl
			return
		}
		atomic.StoreInt32(&self.conn.compressThreshold, int32(c))
	}
	nrPreDigestFields := 2
	if len(cmd.Params) > nrPreDigestFields {
		self.conn.digestFielsLock.Lock()
		defer self.conn.digestFielsLock.Unlock()
		self.conn.digestFields = make([]string, len(cmd.Params)-nrPreDigestFields)
		for i, f := range cmd.Params[nrPreDigestFields:] {
			self.conn.digestFields[i] = f
		}
	}
	return
}
