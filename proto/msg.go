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
	Sender string "s,omitempty"
	SenderService string "v,omitempty"
	Header map[string]string "h,omitempty"
	Body   []byte            "b,omitempty"
}

func (self *Message) Size() int {
	ret := len(self.Body)
	for k, v := range self.Header {
		ret += len(k)
		ret += len(v)
	}
	return ret
}

func (a *Message) Eq(b *Message) bool {
	if len(a.Header) != len(b.Header) {
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
	CMD_DATA = iota
	CMD_EMPTY
	CMD_AUTH
	CMD_AUTHOK
	CMD_ACK
	CMD_BYE
	CMD_INVIS
	CMD_VIS
	CMD_DIGEST_SETTING
	CMD_DIGEST
	CMD_MSG_RETRIEVE
	CMD_FWD
)

type Command struct {
	Type    uint8   "t,omitempty"
	Params  []string "p,omitempty"
	Message *Message "m,omitempty"
}

const (
	maxNrParams = 16
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
	if (self == nil) {
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
	rest = data[idx + 1:]
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

