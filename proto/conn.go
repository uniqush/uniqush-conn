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
	"bufio"
	"encoding/binary"
)

const (
	// AES-256
	sessionKeyLen = 32
	// HMAC-SHA256
	macKeyLen = 32
)

type connection struct {
	conn       net.Conn
	readBuff chan []byte
	writeBuff chan []byte
	maxPktSize int
	err     error
}

var ErrBadSessionKey = errors.New("Bad Sesssion Key")
var ErrBadConnection = errors.New("Bad Connection")

func newConn(resChan <-chan *authResult, maxPktSize int) (conn net.Conn, err error) {
	ret := new(connection)
	ret.resChan = resChan
	ret.maxPktSize = maxPktSize
	ret.readBuff = make(chan []byte)
	ret.writeBuff = make(chan []byte)
	go ret.readWorker(resChan)
	conn = ret
	err = nil
	return
}

func (self *connection) decrypt(buf []byte) ([]byte, error) {
	return buf, nil
}

func (self *connection) encrypt(buf []byte) ([]byte, error) {
	return buf, nil
}

func (self *connection) compress(buf []byte) ([]byte, error) {
	return buf, nil
}

func (self *connection) decompress(buf []byte) ([]byte, error) {
	return buf, nil
}

func (self *connection) readWorker(resChan <-chan *authResult) {
	ar := <-resChan
	sessionKey := make([]byte, sessionKeyLen)
	macKey := make([]byte, macKeyLen)
	if ar != nil {
		if ar.err != nil {
			self.err = ar.err
		} else if len(ar.sessionKey) != sessionKeyLen {
			self.err = ErrBadSessionKey
		} else if ar.c == nil {
			self.err = ErrBadConnection
		} else {
			copy(sessionKey, ar.sessionKey)
			copy(mackey, ar.mackey)
			self.err = nil
			self.conn = ar.c
		}
	}
	if self.err != nil {
		return
	}

	go self.writeWorker(ar)

	reader := bufio.NewReader(ar.c)

	timeout := 10 * time.Second

	for {
		uint32 pkgLen := 0
	}
}

func (self *connection) writeWorker(conf authResult) {
	for buf := range self.writeBuff {
		if buf == nil {
			return
		}
	}
}

func (self *connection) Read(buf []byte) (n int, err error) {
	// TODO read Package
	n, err = self.conn.Read(buf)
	return
}

func (self *connection) Write(buf []byte) (n int, err error) {
	// TODO write Package
	n, err = self.conn.Write(buf)
	return
}

func (self *connection) Close() error {
	err = self.conn.Close()
	return err
}

func (self *connection) LocalAddr() net.Addr {
	return self.conn.LocalAddr()
}

func (self *connection) RemoteAddr() net.Addr {
	return self.conn.RemoteAddr()
}

func (self *connection) SetDeadline(t time.Time) error {
	return self.conn.SetDeadline(t)
}

func (self *connection) SetReadDeadline(t time.Time) error {
	return self.conn.SetReadDeadline(t)
}

func (self *connection) SetWriteDeadline(t time.Time) error {
	return self.conn.SetWriteDeadline(t)
}

