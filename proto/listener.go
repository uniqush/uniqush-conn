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

package proto

import (
	"net"
	"io"
)

type Authenticator interface {
	Authenticate(usr, token string) (bool, error)
}

type serverListener struct {
	listener net.Listener
	auth Authenticator
}

func Listen(listener net.Listener, auth Authenticator) (l net.Listener, err error) {
	ret := new(serverListener)
	ret.listener = listener
	ret.auth = auth
	l = ret
	return
}

func (self *serverListener) Addr() net.Addr {
	return self.listener.Addr()
}

func (self *serverListener) Close() error {
	err := self.listener.Close()
	return err
}

func (self *serverListener) Accept() (conn net.Conn, err error) {
	c, err := self.listener.Accept()
	if err != nil {
		return
	}
	data := make([]byte, 256)
	n, err := io.ReadFull(c, data)
	if n != 256 || err != nil {
		return
	}
	conn = c
	return
}

