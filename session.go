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
	"encoding/binary"
	"sync"
	"sync/atomic"
	"errors"
	"crypto/sha1"
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
	sessionContent_CONTROL
)

var ErrUnauth = errors.New("Unauthorized session")
var ErrBadContentType = errors.New("Bad Content Type")

type ErrorBadProtoImpl struct {
	msg string
}

func (self *ErrorBadProtoImpl) Error() string {
	return "Bad Protocol Implementation: " + self.msg
}

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

// Write() writes the buf to the transport layer.
// It will first compress the data, and then encrypt it.
//
// This method is goroutine-safe
func (self *Session) Write(buf []byte) (n int, err error) {
	if atomic.LoadInt32(&self.state) != sessionState_AUTHED {
		return 0, ErrUnauth
	}
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

// Read() will first read data from the transport layer, 
// then decrypt the data, then decompress it and copy
// the finaly data to the buf.
//
// OK. I am cheating. Read() is actually reading data from
// an internal buffer. All data there has already been
// decrepted & decompressed by another goroutine.
//
// If you cannot understand what I said, simply think it as
// a wrapper of another Reader and can do some magic stuff
// on reading.
//
// This method is goroutine-safe
func (self *Session) Read(buf []byte) (n int, err error) {
	if atomic.LoadInt32(&self.state) != sessionState_AUTHED {
		return 0, ErrUnauth
	}
	n, err = self.buf.Read(buf)
	if err != nil && err != io.EOF {
		return
	}
	for err == io.EOF {
		// If the connection is disconnected, then...
		if atomic.LoadInt32(&self.state) == sessionState_DISCON {
			if n == 0 {
				// return io.EOF if no data is read.
				err = io.EOF
			} else {
				// Otherwise, return nil error with data first,
				// then return an EOF on next call of Read().
				err = nil
			}
			return
		}
		self.buf.WaitForData()
		n, err = self.buf.Read(buf)
		if err != nil && err != io.EOF {
			return
		}
	}
	return
}

func (self *Session) recvLoop() {
	if atomic.LoadInt32(&self.state) != sessionState_AUTHED {
		return
	}
	for {
		rec, err := self.readRecord()
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				atomic.StoreInt32(&self.state, sessionState_DISCON)
			}
			break
		}
		switch rec.contentType {
		case sessionContent_APPDATA:
			buf := rec.buf
			n, err := self.buf.Write(buf)

			for err == ErrFull || n != len(buf) {
				if n > 0 && n < len(buf) {
					buf = buf[:n]
				}
				self.buf.WaitForSpace(len(rec.buf))
				n, err = self.buf.Write(rec.buf)
			}
		}
	}
}

// The Authorizer interface defines one method
// used to authorize the user's name and token
// combination.
type Authorizer interface {
	Authorize(name string, token string) bool
}

func getString(buf []byte) (str string, newbuf []byte) {
	stop := 0
	str = ""
	for i := 0; i < len(buf); i++ {
		if buf[i] == 0 {
			stop = i
			break
		}
	}

	if stop == 0 {
		newbuf = buf
		return
	}

	str = string(buf[:stop])

	newbuf = buf[stop:]
	return
}

// This method should always be called before Read and Write.
// If it returns true, nil, then it means the session now is authorized and ecrypted using
// the new key. Otherwise, any call on Read or Write will return ErrUnauth error.
func (self *Session) WaitAuth(auth Authorizer, timeOut time.Duration) (succ bool, err error) {
	var rec *sessionRecord
	err = nil
	succ = false

	rec, err = self.readRecord()
	if err != nil {
		return
	}

	if rec.contentType != sessionContent_AUTHREQ {
		err = ErrBadContentType
		return
	}

	buf := rec.buf

	// Extract name part
	name, buf := getString(buf)
	if len(name) == 0 {
		err = &ErrorBadProtoImpl{"Empty auth req"}
		return
	}
	token, buf := getString(buf)
	if len(token) == 0 {
		err = &ErrorBadProtoImpl{"Bad token"}
		return
	}

	if len(buf) == 0 {
		err = &ErrorBadProtoImpl{"No key"}
		return
	}

	if auth != nil {
		succ = auth.Authorize(name, token)
	}

	if !succ {
		return
	}

	atomic.StoreInt32(&self.state, sessionState_AUTHED)
	res := new(sessionRecord)
	res.contentType = sessionContent_AUTHRES
	res.version = PROTOCOL_VERSION

	hash := sha1.New()
	hash.Write(rec.buf)
	res.buf = hash.Sum(res.buf)

	err = self.writeRecord(res)
	if err != nil {
		return
	}

	go self.recvLoop()
	return
}

