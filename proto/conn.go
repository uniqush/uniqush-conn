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
	"time"
	"errors"
	"github.com/uniqush/uniqush-conn/streambuf"
)

const (
	// AES-256
	decryptKeyLen = 32
	encryptKeyLen = 32

	// HMAC-SHA256
	macKeyLen = 32
)

type authResult struct {
	encryptKey []byte
	decryptKey []byte
	mackey []byte
	err error
	c net.Conn
	couldClose chan<- bool
}

type connection struct {
	conn       net.Conn
	streamBuf streambuf.StreamBuffer
	err     error
}

var ErrBadEncryptKey = errors.New("Bad Encrypt Key")
var ErrBadDecryptKey = errors.New("Bad Decrypt Key")
var ErrBadConnection = errors.New("Bad Connection")

func newConn(resChan <-chan *authResult, maxPktSize int) (conn net.Conn, err error) {
	ret := new(connection)
	ret.resChan = resChan
	ret.streambuf = streambuf.New(maxPktSize)
	go ret.readWorker(resChan)
	conn = ret
	err = nil
	return
}

func (self *connection) init() error {
	ar := <-self.resChan
	if ar != nil {
		if ar.err != nil {
			self.err = ar.err
		} else if len(ar.encryptKey) != encryptKeyLen {
			self.err = ErrBadEncryptKey
		} else if len(ar.decryptKey) != decryptKeyLen {
			self.err = ErrBadDecryptKey
		} else if ar.c == nil {
			self.err = ErrBadConnection
		} else {
			self.encryptKey = make([]byte, len(ar.encryptKey))
			self.decryptKey = make([]byte, len(ar.decryptKey))
			copy(self.encryptKey, ar.encryptKey)
			copy(self.decryptKey, ar.decryptKey)
			copy(self.mackey, ar.mackey)
			self.err = nil
			self.conn = ar.c
		}
		ar.couldClose <- true
	}
	if self.err != nil {
		return self.err
	}
	return nil
}

func (self *connection) readWorker(resChan <-chan *authResult) {
	ar := <-resChan
	encryptKey := make([]byte, encryptKeyLen)
	decryptKey := make([]byte, decryptKeyLen)
	macKey := make([]byte, macKeyLen)
	if ar != nil {
		if ar.err != nil {
			self.err = ar.err
		} else if len(ar.encryptKey) != encryptKeyLen {
			self.err = ErrBadEncryptKey
		} else if len(ar.decryptKey) != decryptKeyLen {
			self.err = ErrBadDecryptKey
		} else if ar.c == nil {
			self.err = ErrBadConnection
		} else {
			copy(encryptKey, ar.encryptKey)
			copy(decryptKey, ar.decryptKey)
			copy(mackey, ar.mackey)
			self.err = nil
			self.conn = ar.c
		}
		ar.couldClose <- true
	}
}

func (self *connection) Read(buf []byte) (n int, err error) {
	err = self.init()
	if err != nil {
		return
	}
	// TODO read Package
	n, err = self.conn.Read(buf)

	return
}

func (self *connection) Write(buf []byte) (n int, err error) {
	err = self.init()
	if err != nil {
		return
	}
	// TODO write Package
	n, err = self.conn.Write(buf)

	return
}

func (self *connection) Close() error {
	err := self.init()
	if err != nil {
		return err
	}
	err = self.conn.Close()
	return err
}

func (self *connection) LocalAddr() net.Addr {
	err := self.init()
	if err != nil {
		return nil
	}
	return self.conn.LocalAddr()
}

func (self *connection) RemoteAddr() net.Addr {
	err := self.init()
	if err != nil {
		return nil
	}
	return self.conn.RemoteAddr()
}

func (self *connection) SetDeadline(t time.Time) error {
	err := self.init()
	if err != nil {
		return err
	}
	return self.conn.SetDeadline(t)
}

func (self *connection) SetReadDeadline(t time.Time) error {
	err := self.init()
	if err != nil {
		return err
	}
	return self.conn.SetReadDeadline(t)
}

func (self *connection) SetWriteDeadline(t time.Time) error {
	err := self.init()
	if err != nil {
		return err
	}
	return self.conn.SetWriteDeadline(t)
}

