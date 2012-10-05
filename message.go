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
	"labix.org/v2/mgo/bson"
)

const (
	/*
	 * Sent from client.
	 */
	MSGCMD_SUBSCRIBE = iota
	MSGCMD_UNSUBSCRIBE
	MSGCMD_ADDUSER
	MSGCMD_RMUSER
	MSGCMD_AUTH
	MSGCMD_REDIRECT
	MSGCMD_NORMAL_DATA
	MSGCMD_CONTROL
	MSGCMD_CLOSE
)

/*
 * Each message has three parts:
 * command, header and body.
 * They can be arbitrary values. The representation
 * of a message in underlying layers may be different.
 *
 */
type Message struct {
	Command uint32
	Header  map[string]string
	Body    []byte
}

func (self *Message) ToBytes() []byte {
}
