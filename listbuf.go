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
	"container/list"
	"fmt"
	"io"
	"sync"
)

// Used to store sequence of slices into a linked
// list. It implements Reader and Writer interfaces.
type ListBuffer struct {
	bufq *list.List
	lock *sync.Mutex
}

// Return a new empty list buffer
func NewListBuffer() *ListBuffer {
	ret := new(ListBuffer)
	ret.bufq= list.New()
	ret.lock = new(sync.Mutex)
	return ret
}

// Write() implementation.
// NOTE: buf will be *directly* stored to the buffer, not a copy
// of it, meaning changing buf else where will change the content
// of the list buffer.
func (self *ListBuffer) Write(buf []byte) (n int, err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.bufq.PushBack(buf)
	return len(buf), nil
}

func (self *ListBuffer) Read(buf []byte) (n int, err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	n = 0
	err = nil
	for n < len(buf) {
		elem := self.bufq.Front()
		if elem == nil {
			if n == 0 {
				err = io.EOF
			}
			return
		}
		var b []byte
		var ok bool
		if b, ok = elem.Value.([]byte); !ok {
			return 0, fmt.Errorf("Unknown data type in buffer. should be panic")
		}
		self.bufq.Remove(elem)
		c := copy(buf[n:], b)

		if c < len(b) {
			b = b[c:]
			self.bufq.PushFront(b)
		}
		n += c
	}
	return
}

