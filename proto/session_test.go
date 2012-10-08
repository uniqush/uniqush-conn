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
)

func getAuthReqData(name, token string, key []byte) io.Reader {
	buf := make([]byte, len([]byte(name))+len([]byte(token))+len(key)+16)
	ret := bytes.NewBuffer(buf)
	ret.WriteString(name)
	ret.WriteByte(byte(0))
	ret.WriteString(token)
	ret.WriteByte(byte(0))
	ret.Write(key)
	return ret
}

type alwaysPassAuthorizer struct {
}

func (self *alwaysPassAuthorizer) Authorize(name, token string) bool {
	return true
}

func TestAuth(t *testing.T) {
	getAuthReqData("hello", "world", []byte{1, 2, 3})
}
