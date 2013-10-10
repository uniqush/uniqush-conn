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

import "testing"

type redisCacheManager struct {
	db int
}

func (self *redisCacheManager) Name() string {
	return "redis"
}

func (self *redisCacheManager) ClearCache(c Cache) {
	if cache, ok := c.(*redisMessageCache); ok {
		cache.pool.Get().Do("SELECT", self.db)
		cache.pool.Get().Do("FLUSHDB")
	}
}

func (self *redisCacheManager) GetCache() (Cache, error) {
	return NewRedisMessageCache("", "", self.db), nil
}

func TestRedisCache(t *testing.T) {
	testCacheImpl(&redisCacheManager{1}, t)
}

/*
func TestGetSetMessage(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache := getCache()
	defer clearDb()
	srv := "srv"
	usr := "usr"

	ids := make([]string, N)

	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("Set error: %v", err)
			return
		}
		ids[i] = id
	}
	for i, msg := range msgs {
		m, err := cache.Get(srv, usr, ids[i])
		if err != nil {
			t.Errorf("Get error: %v", err)
			return
		}
		if !m.Message.Eq(msg.Message) {
			t.Errorf("%vth message does not same", i)
		}
	}
}

func TestGetSetMessageTTL(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache := getCache()
	defer clearDb()
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

func TestCacheThenRetrieveAll(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache := getCache()
	defer clearDb()
	srv := "srv"
	usr := "usr"

	ids := make([]string, N)

	for i, msg := range msgs {
		id, err := cache.CacheMessage(srv, usr, msg, 0*time.Second)
		if err != nil {
			t.Errorf("Set error: %v", err)
			return
		}
		ids[i] = id
	}

	retrievedMsgs, err := cache.GetCachedMessages(srv, usr)
	if err != nil {
		t.Errorf("Set error: %v", err)
		return
	}
	for i, id := range ids {
		if retrievedMsgs[i].Id != id {
			t.Errorf("retrieved different ids: %v != %v", retrievedMsgs, ids)
			return
		}
	}
}

func TestGetNonExistMsg(t *testing.T) {
	cache := getCache()
	defer clearDb()
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

func TestCacheThenRetrieveAllWithTTL(t *testing.T) {
	N := 10
	msgs := multiRandomMessage(N)
	cache := getCache()
	defer clearDb()
	srv := "srv"
	usr := "usr"

	ids := make([]string, N)

	ttl := 0
	nrDead := 2
	for i, msg := range msgs {
		if i == len(msgs)-nrDead {
			ttl = 1
		}
		id, err := cache.CacheMessage(srv, usr, msg, time.Duration(ttl)*time.Second)
		if err != nil {
			t.Errorf("Set error: %v", err)
			return
		}
		ids[i] = id
	}
	time.Sleep(2 * time.Second)
	retrievedMsgs, err := cache.GetCachedMessages(srv, usr)
	if err != nil {
		t.Errorf("Set error: %v", err)
		return
	}
	if len(retrievedMsgs) != len(msgs)-nrDead {
		t.Errorf("retrieved %v objects", len(retrievedMsgs))
		return
	}
	for i, id := range ids[:len(msgs)-nrDead] {
		if retrievedMsgs[i].Id != id {
			t.Errorf("retrieved different ids: %v != %v", retrievedMsgs, ids)
			return
		}
	}
}
*/
