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

package main

import (
	"io"
	"time"
	"encoding.binary"
)

const (
	sessionState_UNAUTH = iota
	sessionState_AUTHED
)

const (
	sessionContent_AUTHREQ = iota
	sessionContent_AUTHRES
	sessionContent_APP
)

type Session struct {
	state int
	transport io.ReadWriteCloser
}

type sessionRecord struct {
	contentType uint8
	version uint8
	buf []byte
}

func NewSession(transport io.ReadWriteCloser) *Session {
	ret := new(Session)
	ret.transport = transport
	ret.state = sessionState_UNAUTH
	return ret
}

func (self *Session) readRecord() (rec *sessionRecord, err error) {
	rec = new(sessionRecord)
	err = binary.Read(self.transport, binary.LittleEndian, &rec.contentType)
	if err != nil {
		rec = nil
		return
	}
	err = binary.Read(self.transport, binary.LittleEndian, &rec.version)
	if err != nil {
		rec = nil
		return
	}
	var length uint16
	err = binary.Read(self.transport, binary.LittleEndian, &length)
	if err != nil {
		rec = nil
		return
	}
	rec.buf = make([]byte, length)
	_, err = io.ReadFull(self.transport, rec.buf)
	if err != nil {
		rec = nil
		return
	}
	err = nil
	return
}

func (self *Session) writeRecord(rec *sessionRecord) error {
	var err error
	err = binary.Write(self.transport, binary.LittleEndian, rec.contentType)
	if err != nil {
		return err
	}
	err = binary.Write(self.transport, binary.LittleEndian, rec.version)
	if err != nil {
		return err
	}
	var length uint16
	length = uint16(len(rec.buf))
	err = binary.Write(self.transport, binary.LittleEndian, length)
	if err != nil {
		return err
	}

	_, err = transport.Write(rec.buf)
	return err
}

func (self *Session) WaitAuth(timeOut time.Duration) (succ bool, err error) {
}

