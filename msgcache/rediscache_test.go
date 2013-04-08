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

package msgcache

import (
	"testing"
	"github.com/uniqush/uniqush-conn/proto"
	"crypto/rand"
	"io"
)

func randomMessage() *proto.Message {
	msg := new(proto.Message)
	msg.Body = make([]byte, 10)
	io.ReadFull(rand.Reader, msg.Body)
	msg.Header = make(map[string]string, 2)
	msg.Header["aaa"] = "hello"
	msg.Header["aa"] = "hell"
	return msg
}

func multiRandomMessage(N int) []*proto.Message {
	msgs := make([]*proto.Message, N)
	for i := 0; i < N; i++ {
		msgs[i] = randomMessage()
	}
	return msgs
}

func getCache() Cache {
	return NewRedisMessageCache("", "", 1)
}

func TestEnqueueDequeue(t *testing.T) {
	msgs := multiRandomMessage(10)
	cache := getCache()
	srv := "srv"
	usr := "usr"
	for _, msg := range msgs {
		_, err := cache.Enqueue(srv, usr, msg)
		if err != nil {
			t.Errorf("Enqueue error: %v", err)
			return
		}
	}
	for i, msg := range msgs {
		m, err := cache.Dequeue(srv, usr)
		if err != nil {
			t.Errorf("Dequeue error: %v", err)
			return
		}
		if !m.Eq(msg) {
			t.Errorf("%vth message does not same", i)
		}
	}
}

