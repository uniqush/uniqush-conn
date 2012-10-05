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
	//"time"
	"encoding/binary"
	"sync"
)

const (
	PROTOCOL_VERSION = 1
)

const (
	sessionState_UNAUTH = iota
	sessionState_AUTHED
	sessionState_DISCON
)

const (
	sessionContent_AUTHREQ = iota
	sessionContent_AUTHRES
	sessionContent_APPDATA
)

// A session deals with:
// - Authentication
// - Encryption
// - Compression
//
// It provides a ReadWriteCloser implementation.
// Read, Write could be safely called in parallel.
type Session struct {
	state int32
	transport io.ReadWriteCloser
	buf *ListBuffer
	writeLock *sync.Mutex
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

	// The buffer will hold at most 8K bytes of data
	ret.buf = NewListBuffer(8192)
	ret.writeLock = new(sync.Mutex)
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
	self.writeLock.Lock()
	defer self.writeLock.Unlock()
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

	_, err = self.transport.Write(rec.buf)
	return err
}

func (self *Session) Write(buf []byte) (n int, err error) {
	rec := new(sessionRecord)
	rec.contentType = sessionContent_APPDATA
	rec.version = PROTOCOL_VERSION
	rec.buf = buf
	n = 0
	err = self.writeRecord(rec)
	if err != nil {
		return
	}
	n = len(buf)
	return
}

func (self *Session) Read(buf []byte) (n int, err error) {
	n, err = self.buf.Read(buf)
	for err == io.EOF {
		self.buf.WaitForData()
		n, err = self.buf.Read(buf)
	}
	return
}

func (self *Session) recvLoop() {
	for {
		rec, err := self.readRecord()
		if err != nil {
			break
		}
		switch rec.contentType {
		case sessionContent_APPDATA:
			_, err := self.buf.Write(rec.buf)
			for err == ErrFull {
				self.buf.WaitForSpace(len(rec.buf))
				_, err = self.buf.Write(rec.buf)
			}
		}
	}
}

// func (self *Session) WaitAuth(timeOut time.Duration) (succ bool, err error) {
//}

