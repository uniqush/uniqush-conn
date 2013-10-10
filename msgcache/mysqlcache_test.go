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
	"time"

	"testing"
)

func getMysqlCache() (*mysqlMessageCache, error) {
	return NewSQLMessageCache("uniqush", "uniqush-pass", "127.0.0.1:3306", "uniqush")
}

func TestGetSetMessagesToMysql(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := getMysqlCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
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
		if !m.Message.Eq(msg.Message) {
			t.Errorf("%vth message does not same", i)
		}
	}
}

func TestGetSetMessageTTLMysql(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache, err := getMysqlCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
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

func TestGetNonExistMsgMysql(t *testing.T) {
	cache, err := getMysqlCache()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
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
