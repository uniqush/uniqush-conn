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
	"net"
)

type Conn interface {
	proto.Conn
}

type serverConn struct {
	proto.Conn
	cmdio *proto.CommandIO
}

func (self *serverConn) WriteMessage(msg *proto.Message, compress, encrypt bool) error {
	return self.Conn.WriteMessage(msg, compress, encrypt)
}

func (self *serverConn) ProcessCommand(cmd *proto.Command) error {
	return nil
}

func NewConn(cmdio *proto.CommandIO, service, username string, conn net.Conn) Conn {
	sc := new(serverConn)
	sc.cmdio = cmdio
	c := proto.NewConn(cmdio, service, username, conn, sc)
	sc.Conn = c
	return sc
}

