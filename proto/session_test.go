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
	"bytes"
	"io"
	"testing"
	"time"
	"encoding/binary"
)

func getAuthReqData(name, token string, key []byte) io.Reader {
	buf := make([]byte, 0, len([]byte(name))+len([]byte(token))+len(key)+16)
	ret := bytes.NewBuffer(buf)
	var ui8 uint8
	ui8 = sessionContent_AUTHREQ
	binary.Write(ret, binary.LittleEndian, ui8)
	ui8 = PROTOCOL_VERSION
	binary.Write(ret, binary.LittleEndian, ui8)

	var ui16 uint16
	ui16 = uint16(len([]byte(name)) + 1 + len([]byte(token)) + 1 + len(key))
	binary.Write(ret, binary.LittleEndian, ui16)
	ret.WriteString(name)
	ret.WriteByte(byte(0))
	ret.WriteString(token)
	ret.WriteByte(byte(0))
	ret.Write(key)
	return ret
}

type fakeAuthorizer struct {
	pass bool
}

func newFakeAuthorizer(pass bool) Authorizer {
	ret := new(fakeAuthorizer)
	ret.pass = pass
	return ret
}

func (self *fakeAuthorizer) Authorize(name, token string) bool {
	return self.pass
}

type nopCloser struct {
}

func (self *nopCloser) Close() error {
	return nil
}

type rwcCombo struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
}

func (self *rwcCombo) Read(buf []byte) (int, error) {
	return self.reader.Read(buf)
}

func (self *rwcCombo) Write(buf []byte) (int, error) {
	return self.writer.Write(buf)
}

func (self *rwcCombo) Close() error {
	return self.closer.Close()
}

func testAuth(pass bool) error {
	reader := getAuthReqData("hello", "world", []byte{1, 2, 3})
	writer := bytes.NewBuffer(make([]byte, 0, 128))
	rwc := &rwcCombo{reader: reader, writer: writer, closer: &nopCloser{}}
	session := NewSession(rwc)
	auth := newFakeAuthorizer(pass)
	to := 0 * time.Second
	succ, err := session.WaitAuth(auth, to)
	if succ != pass {
		return err
	}
	return nil
}

func TestAuth(t *testing.T) {
	err := testAuth(true)
	if err != nil {
		t.Errorf("should pass but not: %v", err)
	}
	err = testAuth(false)
	if err != nil {
		t.Errorf("should not pass but passed: %v", err)
	}
}
