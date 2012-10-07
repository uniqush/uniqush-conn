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
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

var ErrFull = errors.New("Full")

// Used to store sequence of slices into a linked
// list. It implements Reader and Writer interfaces.
type ListBuffer struct {
	bufq     *list.List
	lock     *sync.Mutex
	size     int32
	capacity int32

	hasSpace      *sync.Cond
	spaceCondLock *sync.Mutex
	hasData       *sync.Cond
	dataCondLock  *sync.Mutex
}

// Return a new empty list buffer
// capacity: the capacity of the buffer.
// <= 0 means unlimited.
func NewListBuffer(capacity int) *ListBuffer {
	ret := new(ListBuffer)
	ret.bufq = list.New()
	ret.lock = new(sync.Mutex)
	ret.size = 0
	ret.capacity = int32(capacity)

	ret.spaceCondLock = new(sync.Mutex)
	ret.hasSpace = sync.NewCond(ret.spaceCondLock)
	ret.dataCondLock = new(sync.Mutex)
	ret.hasData = sync.NewCond(ret.dataCondLock)
	return ret
}

// Return if there is empty space in the buffer.
// Otherwise, it will block the whole goroutine.
//
// Even if this method returned, it does not necessary mean
// that the next Write() will be success.
func (self *ListBuffer) WaitForSpace(size int) {
	self.spaceCondLock.Lock()
	defer self.spaceCondLock.Unlock()

	if self.capacity < 0 || atomic.LoadInt32(&self.size) < self.capacity {
		return
	}
	self.hasSpace.Wait()
	return
}

// Return if there is data in the buffer.
// Otherwise, it will block the whole goroutine.
//
// Even if this method returned, it does not necessary mean
// that the next Read() will never return io.EOF.
func (self *ListBuffer) WaitForData() {
	self.dataCondLock.Lock()
	defer self.dataCondLock.Unlock()

	if atomic.LoadInt32(&self.size) > 0 {
		return
	}
	self.hasData.Wait()
	return
}

// Write() implementation.
// NOTE: buf will be *directly* stored to the buffer, not a copy
// of it, meaning changing buf else where will change the content
// of the list buffer.
//
// Returns ErrFull if there is no space to store one single byte.
func (self *ListBuffer) Write(buf []byte) (n int, err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.dataCondLock.Lock()
	defer self.dataCondLock.Unlock()

	if self.capacity > 0 && atomic.LoadInt32(&self.size) >= self.capacity {
		return 0, ErrFull
	}

	n = len(buf)
	if self.capacity > 0 {
		free := self.capacity - atomic.LoadInt32(&self.size)
		if n > int(free) {
			n = int(free)
		}
	}
	self.bufq.PushBack(buf[:n])

	atomic.AddInt32(&self.size, int32(n))

	self.hasData.Broadcast()
	return
}

// Read() implementation.
//
// If the ListBuffer is empty, Read() returns io.EOF
func (self *ListBuffer) Read(buf []byte) (n int, err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.spaceCondLock.Lock()
	defer self.spaceCondLock.Unlock()
	n = 0
	err = nil
	for n < len(buf) {
		elem := self.bufq.Front()
		if elem == nil {
			if n == 0 {
				err = io.EOF
			}
			break
		}
		var b []byte
		var ok bool
		if b, ok = elem.Value.([]byte); !ok {
			err = fmt.Errorf("Unknown data type in buffer. should be panic")
			break
		}
		self.bufq.Remove(elem)
		c := copy(buf[n:], b)

		if c < len(b) {
			b = b[c:]
			self.bufq.PushFront(b)
		}
		n += c
		atomic.AddInt32(&self.size, -int32(c))
	}

	if n > 0 {
		self.hasSpace.Broadcast()
	}
	return
}

// Returns the size of the data inside the buffer
func (self *ListBuffer) Size() int {
	self.lock.Lock()
	defer self.lock.Unlock()
	s := atomic.LoadInt32(&self.size)
	return int(s)
}
