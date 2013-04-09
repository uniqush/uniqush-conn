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
	"crypto/rand"
	"github.com/garyburd/redigo/redis"
	"github.com/uniqush/uniqush-conn/proto"
	"io"
	"testing"
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
	db := 1
	c, _ := redis.Dial("tcp", "localhost:6379")
	c.Do("SELECT", db)
	c.Do("FLUSHDB")
	c.Close()
	return NewRedisMessageCache("", "", db)
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

func TestEnqueueDel(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	idMap := make(map[string]*proto.Message, N)
	cache := getCache()
	srv := "srv"
	usr := "usr"

	for _, msg := range msgs {
		id, err := cache.Enqueue(srv, usr, msg)
		if err != nil {
			t.Errorf("Enqueue error: %v", err)
			return
		}
		idMap[id] = msg
	}

	for id, msg := range idMap {
		m, err := cache.DelFromQueue(srv, usr, id)
		if err != nil {
			t.Errorf("Del error: %v", err)
			return
		}
		if !m.Eq(msg) {
			t.Errorf("message %v does not same", id)
		}
	}
}
