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
	"github.com/uniqush/uniqush-conn/rpc"
	"io"
	"testing"
	"time"
)

func randomMessage() *rpc.Message {
	msg := new(rpc.Message)
	msg.Body = make([]byte, 10)
	io.ReadFull(rand.Reader, msg.Body)
	msg.Header = make(map[string]string, 2)
	msg.Header["aaa"] = "hello"
	msg.Header["aa"] = "hell"
	return msg
}

func multiRandomMessage(N int) []*rpc.MessageContainer {
	msgs := make([]*rpc.MessageContainer, N)
	for i := 0; i < N; i++ {
		msgs[i] = new(rpc.MessageContainer)
		msgs[i].Message = randomMessage()
	}
	return msgs
}

type cacheManager interface {
	GetCache() (cache Cache, err error)
	ClearCache(cache Cache)
	Name() string
}

func testGetSetMessages(m cacheManager, t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer m.ClearCache(cache)
	srv := "myservice"
	usr := "user2"

	ids := make([]string, N, N)
	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
		ids[i] = id
	}
	for i, msg := range msgs {
		m, err := cache.Get(srv, usr, ids[i])
		if err != nil {
			t.Errorf("Del error: %v", err)
			return
		}
		if m.Id != ids[i] {
			t.Errorf("ask id %v; but get %v", ids[i], m.Id)
		}
		if !m.Message.Eq(msg.Message) {
			t.Errorf("%vth message does not same", i)
		}
	}
}

func testGetSetMessagesTTL(m cacheManager, t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer m.ClearCache(cache)
	srv := "srv"
	usr := "usr"

	ids := make([]string, N)

	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 1*time.Second)
		if err != nil {
			t.Errorf("Set error: %v", err)
			return
		}
		ids[i] = id
	}
	time.Sleep(2 * time.Second)
	for i, id := range ids {
		m, err := cache.Get(srv, usr, id)
		if err != nil {
			t.Errorf("Get error: %v", err)
			return
		}
		if m != nil {
			t.Errorf("%vth message should be deleted", i)
		}
	}
}

func testGetNonExistMessage(m cacheManager, t *testing.T) {
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer m.ClearCache(cache)
	srv := "srv"
	usr := "usr"

	msg, err := cache.Get(srv, usr, "wont-be-a-good-message-id")
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	if msg != nil {
		t.Errorf("should be nil message")
		return
	}
}

func hasMessage(mc *rpc.MessageContainer, mclist []*rpc.MessageContainer) bool {
	for _, rmc := range mclist {
		if mc.Message.Eq(rmc.Message) {
			return true
		}
	}
	return false
}

func testRetrieveAllMessages(m cacheManager, t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer m.ClearCache(cache)
	srv := "myservice"
	usr := "user2"

	ids := make([]string, N, N)
	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
		ids[i] = id
	}

	rmsgs, err := cache.RetrieveAllSince(srv, usr, time.Time{})
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}
	if len(rmsgs) != len(msgs) {
		t.Errorf("Length is not the same: %v != %v", len(rmsgs), len(msgs))
	}
	for i, msg := range rmsgs {
		// All messages are stored at almost the same time in terms of seconds.
		// So their order may not be reserved.
		if !hasMessage(msg, msgs) {
			t.Errorf("%vth message cannot be found: %+v", i, msg.Message)
		}
	}
}

func testRetrieveAllMessagesSinceSometime(m cacheManager, t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer m.ClearCache(cache)
	srv := "myservice"
	usr := "user2"

	// messages inserted in this loop will be ignored
	for _, msg := range msgs {
		_, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
	}

	time.Sleep(2 * time.Second)
	since := time.Now()
	msgs = multiRandomMessage(N)
	ids := make([]string, N, N)

	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
		ids[i] = id
	}

	rmsgs, err := cache.RetrieveAllSince(srv, usr, since)
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}
	if len(rmsgs) != len(msgs) {
		t.Errorf("Length is not the same: %v != %v", len(rmsgs), len(msgs))
	}

	for i, msg := range rmsgs {
		if !hasMessage(msg, msgs) {
			t.Errorf("%vth message cannot be found: %+v", i, msg.Message)
		}
	}
}

func testRetrieveAllMessagesTTL(m cacheManager, t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer m.ClearCache(cache)
	srv := "myservice"
	usr := "user2"

	// messages inserted in this loop will be ignored
	for _, msg := range msgs {
		_, err := cache.CacheMessage(srv, usr, msg, 1*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
	}

	time.Sleep(2 * time.Second)
	msgs = multiRandomMessage(N)
	ids := make([]string, N, N)

	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
		ids[i] = id
	}

	rmsgs, err := cache.RetrieveAllSince(srv, usr, time.Time{})
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}
	if len(rmsgs) != len(msgs) {
		t.Errorf("Length is not the same: %v != %v", len(rmsgs), len(msgs))
	}

	for i, msg := range rmsgs {
		if !hasMessage(msg, msgs) {
			t.Errorf("%vth message cannot be found: %+v", i, msg.Message)
		}
	}
}

func testRetrieveAllReserveOrder(m cacheManager, t *testing.T) {
	N := 2
	msgs := multiRandomMessage(N)
	cache, err := m.GetCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer m.ClearCache(cache)
	srv := "myservice"
	usr := "user2"

	// messages inserted in this loop will be ignored
	for _, msg := range msgs {
		_, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("set error: %v", err)
			return
		}
		time.Sleep(1 * time.Second)
	}

	rmsgs, err := cache.RetrieveAllSince(srv, usr, time.Time{})
	if err != nil {
		t.Errorf("error: %v", err)
		return
	}
	if len(rmsgs) != len(msgs) {
		t.Errorf("Length is not the same: %v != %v", len(rmsgs), len(msgs))
	}

	for i, msg := range rmsgs {
		m := msgs[i]
		if !m.Message.Eq(msg.Message) {
			t.Errorf("%vth message is corrupted: retrieved %+v; should be %+v", i, msg.Message, m.Message)
		}
	}
}

func testCacheImpl(m cacheManager, t *testing.T) {
	testGetSetMessages(m, t)
	testGetNonExistMessage(m, t)
	testGetSetMessagesTTL(m, t)
	testRetrieveAllMessages(m, t)
	testRetrieveAllMessagesSinceSometime(m, t)
	testRetrieveAllMessagesTTL(m, t)
	testRetrieveAllReserveOrder(m, t)
}
