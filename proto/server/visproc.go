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
	"sync/atomic"

	"github.com/uniqush/uniqush-conn/proto"
)

type visibilityProcessor struct {
	conn *serverConn
}

func (self *visibilityProcessor) ProcessCommand(cmd *proto.Command) (msg *proto.Message, err error) {
	fmt.Printf("visible: %+v\n", cmd)
	if cmd == nil || cmd.Type != proto.CMD_SET_VISIBILITY {
		return
	}
	if len(cmd.Params) < 1 {
		err = proto.ErrBadPeerImpl
		return
	}
	if cmd.Params[0] == "0" {
		atomic.StoreInt32(&self.conn.visible, 0)
	} else if cmd.Params[0] == "1" {
		atomic.StoreInt32(&self.conn.visible, 1)
	}
	return
}
