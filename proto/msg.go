/*
 * Copyright 2013 Nan Deng
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
	"errors"
)

type Message struct {
	Id            string            "i,omitempty"
	Sender        string            "s,omitempty"
	SenderService string            "v,omitempty"
	Header        map[string]string "h,omitempty"
	Body          []byte            "b,omitempty"
}

func (self *Message) IsEmpty() bool {
	return len(self.Header) == 0 && len(self.Body) == 0
}

func (self *Message) Size() int {
	ret := len(self.Body)
	for k, v := range self.Header {
		ret += len(k) + 1
		ret += len(v) + 1
	}
	ret += 8
	return ret
}

func (a *Message) Eq(b *Message) bool {
	if len(a.Header) != len(b.Header) {
		return false
	}
	if a.Id != b.Id {
		return false
	}
	for k, v := range a.Header {
		if bv, ok := b.Header[k]; ok {
			if bv != v {
				return false
			}
		} else {
			return false
		}
	}
	return bytesEq(a.Body, b.Body)
}

const (
	cmdflag_COMPRESS = 1 << iota
	cmdflag_ENCRYPT
	cmdflag_NEEDACK
)

const (
	// Params:
	// 0. [optional] The Id of the message
	CMD_DATA = iota

	// Params:
	// 0. [optional] The Id of the message
	CMD_EMPTY

	// Sent from client.
	//
	// Params
	// 0. service name
	// 1. username
	CMD_AUTH

	CMD_AUTHOK
	CMD_BYE

	// Sent from client.
	// Telling the server about its perference.
	//
	// Params:
	// 0. Digest threshold: -1 always send message directly; Empty: not change
	// 1. Compression threshold: -1 always compress the data; Empty: not change
	// 2. Encryption (1 - encrypt; 0 - no ecnrypt; others - not change)
	// >3. [optional] Digest fields
	CMD_SETTING

	// Sent from server.
	// Telling the client an
	// arrival of a message.
	//
	// Params:
	// 0. Size of the message
	// 1. The id of the message
	//
	// Message.Header:
	// Other digest info
	CMD_DIGEST

	// Sent from client.
	// Telling the server which cached
	// message it wants to retrive.
	//
	// Params:
	// 0. The message id
	CMD_MSG_RETRIEVE

	// Sent from client.
	// Telling the server to forward a
	// message to another user.
	//
	// Params:
	// 0. Reciever's name
	// 1. [optional] Reciever's service name.
	//    If empty, then same service as the client
	CMD_FWD_REQ

	// Sent from server.
	// Telling the client the mssage
	// is originally from another user.
	//
	// Params:
	// 0. Sender's name
	// 1. [optional] Sender's service name.
	//    If empty, then same service as the client
	// 2. [optional] The Id of the message in the cache.
	CMD_FWD
)

type Command struct {
	Type    uint8    "t,omitempty"
	Params  []string "p,omitempty"
	Message *Message "m,omitempty"
}

const (
	maxNrParams  = 16
	maxNrHeaders = 0x0000FFFF
)

var ErrTooManyParams = errors.New("Too many parameters: 16 max")
var ErrTooManyHeaders = errors.New("Too many headers: 4096 max")

// | Type | NrParams | Reserved | NrHeaders | Params | Header | Body |
//
// Type: 8 bit
// NrParams: 4 bit
// Reserved: 4 bit
// NrHeaders: 16 bit Byte order: MSB | LSB. i.e. big endian
// Params: list of strings. each string ends with \0. (ACII 0)
// Header: list of string pairs. each string ends with \0. (ACII 0)
func (self *Command) Marshal() (data []byte, err error) {
	if self == nil {
		return
	}
	nrParams := len(self.Params)
	nrHeaders := 0
	if nrParams > maxNrParams {
		err = ErrTooManyParams
		return
	}
	if self.Message != nil {
		nrHeaders = len(self.Message.Header)
		if nrHeaders > maxNrHeaders {
			err = ErrTooManyHeaders
			return
		}
	}

	data = make([]byte, 4, 1024)
	data[0] = byte(self.Type)

	data[1] = byte(0x0000000F & nrParams)
	data[1] = data[1] << 4

	data[2] = byte((0xFF00 & uint16(nrHeaders)) >> 8)
	data[3] = byte(0x00FF & uint16(nrHeaders))

	for _, param := range self.Params {
		data = append(data, []byte(param)...)
		data = append(data, byte(0))
	}
	if self.Message == nil {
		return
	}

	for k, v := range self.Message.Header {
		data = append(data, []byte(k)...)
		data = append(data, byte(0))
		data = append(data, []byte(v)...)
		data = append(data, byte(0))
	}

	if len(self.Message.Body) > 0 {
		data = append(data, self.Message.Body...)
	}
	return
}

func (self *Command) eq(cmd *Command) bool {
	if self.Type != cmd.Type {
		return false
	}
	if len(self.Params) != len(cmd.Params) {
		return false
	}
	for i, p := range self.Params {
		if cmd.Params[i] != p {
			return false
		}
	}
	if self.Message == nil {
		return true
	}
	return self.Message.Eq(cmd.Message)
}

var ErrMalformedCommand = errors.New("malformed command")

func cutString(data []byte) (str, rest []byte, err error) {
	var idx int
	var d byte
	idx = -1
	for idx, d = range data {
		if d == 0 {
			break
		}
	}
	if idx < 0 {
		err = ErrMalformedCommand
		return
	}
	str = data[:idx]
	rest = data[idx+1:]
	return
}

func UnmarshalCommand(data []byte) (cmd *Command, err error) {
	if len(data) < 4 {
		return
	}
	cmd = new(Command)
	cmd.Type = data[0]
	nrParams := int(data[1] >> 4)
	nrHeaders := int((uint16(data[2]) << 8) | (uint16(data[3])))

	data = data[4:]
	if nrParams > 0 {
		cmd.Params = make([]string, nrParams)
		var str []byte
		for i := 0; i < nrParams; i++ {
			str, data, err = cutString(data)
			if err != nil {
				return
			}
			cmd.Params[i] = string(str)
		}
	}
	var msg *Message
	msg = nil
	if nrHeaders > 0 {
		msg = new(Message)
		msg.Header = make(map[string]string, nrHeaders)
		var key []byte
		var value []byte
		for i := 0; i < nrHeaders; i++ {
			key, data, err = cutString(data)
			if err != nil {
				return
			}
			value, data, err = cutString(data)
			if err != nil {
				return
			}
			msg.Header[string(key)] = string(value)
		}
	}
	if len(data) > 0 {
		if msg == nil {
			msg = new(Message)
		}
		msg.Body = data
	}
	if msg != nil {
		cmd.Message = msg
	}
	return
}
