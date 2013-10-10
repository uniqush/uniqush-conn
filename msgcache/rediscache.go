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
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/uniqush/uniqush-conn/rpc"
	"math/rand"
	"strconv"
	"time"
)

type redisCacheManager struct {
}

func (self *redisCacheManager) GetCache(addr, username, password, database string) (Cache, error) {
	db := 0
	if len(database) > 0 {
		var err error
		db, err = strconv.Atoi(database)
		if err != nil {
			return nil, fmt.Errorf("bad database %v: %v", database, err)
		}
	}

	return NewRedisMessageCache(addr, password, db), nil
}

func (self *redisCacheManager) Engine() string {
	return "redis"
}

type redisMessageCache struct {
	pool *redis.Pool
}

func NewRedisMessageCache(addr, password string, db int) Cache {
	if len(addr) == 0 {
		addr = "localhost:6379"
	}
	if db < 0 {
		db = 0
	}

	dial := func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		if len(password) > 0 {
			if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
		}
		if _, err := c.Do("SELECT", db); err != nil {
			c.Close()
			return nil, err
		}
		return c, err
	}
	testOnBorrow := func(c redis.Conn, t time.Time) error {
		_, err := c.Do("PING")
		return err
	}

	pool := &redis.Pool{
		MaxIdle:      3,
		IdleTimeout:  240 * time.Second,
		Dial:         dial,
		TestOnBorrow: testOnBorrow,
	}

	ret := new(redisMessageCache)
	ret.pool = pool
	return ret
}

func randomId() string {
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), rand.Int63())
}

func (self *redisMessageCache) CacheMessage(service, username string, msg *rpc.MessageContainer, ttl time.Duration) (id string, err error) {
	id = randomId()
	err = self.set(service, username, id, msg, ttl)
	if err != nil {
		id = ""
		return
	}
	return
}

func msgKey(service, username, id string) string {
	return fmt.Sprintf("mcache:%v:%v:%v", service, username, id)
}

func msgKeyPattern(service, username string) string {
	return fmt.Sprintf("mcache:%v:%v:*", service, username)
}

func msgQueueKey(service, username string) string {
	return fmt.Sprintf("mqueue:%v:%v", service, username)
}

func msgWeightKey(service, username, id string) string {
	return fmt.Sprintf("w_mcache:%v:%v:%v", service, username, id)
}

func msgWeightPattern(service, username string) string {
	return fmt.Sprintf("w_mcache:%v:%v:*", service, username)
}

func counterKey(service, username string) string {
	return "msgCounter"
}

func msgMarshal(msg *rpc.MessageContainer) (data []byte, err error) {
	data, err = json.Marshal(msg)
	return
}

func msgUnmarshal(data []byte) (msg *rpc.MessageContainer, err error) {
	msg = new(rpc.MessageContainer)
	err = json.Unmarshal(data, msg)
	if err != nil {
		msg = nil
		return
	}
	return
}

func (self *redisMessageCache) set(service, username, id string, msg *rpc.MessageContainer, ttl time.Duration) error {
	msg.Id = id
	msg.Birthday = time.Now()
	key := msgKey(service, username, id)
	conn := self.pool.Get()
	defer conn.Close()

	data, err := msgMarshal(msg)
	if err != nil {
		return err
	}

	/*
		reply, err := conn.Do("INCR", counterKey(service, username))
		if err != nil {
			return err
		}

		weight, err := redis.Int64(reply, err)
		if err != nil {
			return err
		}
	*/
	weight := time.Now().Unix()
	wkey := msgWeightKey(service, username, id)

	err = conn.Send("MULTI")
	if err != nil {
		return err
	}

	if ttl.Seconds() <= 0.0 {
		err = conn.Send("SET", key, data)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
		err = conn.Send("SET", wkey, weight)
	} else {
		err = conn.Send("SETEX", key, int64(ttl.Seconds()), data)
		if err != nil {
			conn.Do("DISCARD")
			return err
		}
		err = conn.Send("SETEX", wkey, int64(ttl.Seconds()), weight)
	}
	if err != nil {
		conn.Do("DISCARD")
		return err
	}
	msgQK := msgQueueKey(service, username)
	err = conn.Send("SADD", msgQK, id)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}
	_, err = conn.Do("EXEC")
	if err != nil {
		return err
	}
	return nil
}

func (self *redisMessageCache) Get(service, username, id string) (msg *rpc.MessageContainer, err error) {
	key := msgKey(service, username, id)
	conn := self.pool.Get()
	defer conn.Close()

	reply, err := conn.Do("GET", key)
	if err != nil {
		return
	}
	if reply == nil {
		return
	}
	data, err := redis.Bytes(reply, err)
	if err != nil {
		return
	}
	msg, err = msgUnmarshal(data)
	return
}

/*
 * We may not need Delete
func (self *redisMessageCache) Del(service, username, id string) error {
	key := msgKey(service, username, id)
	wkey := msgWeightKey(service, username, id)
	conn := self.pool.Get()
	defer conn.Close()

	err := conn.Send("MULTI")
	if err != nil {
		return err
	}
	err = conn.Send("DEL", key)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}
	err = conn.Send("DEL", wkey)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}
	msgQK := msgQueueKey(service, username)
	err = conn.Send("SREM", msgQK, id)
	if err != nil {
		conn.Do("DISCARD")
		return err
	}
	_, err = conn.Do("EXEC")
	if err != nil {
		return err
	}
	return nil
}
*/

/*
 * We may not need get then delete.
func (self *redisMessageCache) GetThenDel(service, username, id string) (msg *proto.Message, err error) {
	key := msgKey(service, username, id)
	wkey := msgWeightKey(service, username, id)
	conn := self.pool.Get()
	defer conn.Close()

	err = conn.Send("MULTI")
	if err != nil {
		return
	}
	err = conn.Send("GET", key)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	err = conn.Send("DEL", key)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	err = conn.Send("DEL", wkey)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	msgQK := msgQueueKey(service, username)
	err = conn.Send("SREM", msgQK, id)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	reply, err := conn.Do("EXEC")
	if err != nil {
		return
	}

	bulkReply, err := redis.Values(reply, err)
	if err != nil {
		return
	}
	if len(bulkReply) != 4 {
		return
	}
	if bulkReply[0] == nil {
		return
	}
	data, err := redis.Bytes(bulkReply[0], err)
	if err != nil {
		return
	}
	if len(data) == 0 {
		return
	}
	msg, err = msgUnmarshal(data)
	return
}
*/

func (self *redisMessageCache) RetrieveAllSince(service, username string, since time.Time) (msgs []*rpc.MessageContainer, err error) {
	msgQK := msgQueueKey(service, username)
	conn := self.pool.Get()
	defer conn.Close()

	err = conn.Send("MULTI")
	if err != nil {
		return
	}
	err = conn.Send("SORT", msgQK,
		"BY",
		msgWeightPattern(service, username),
		"GET",
		msgKeyPattern(service, username))
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	err = conn.Send("SORT", msgQK,
		"BY",
		msgWeightPattern(service, username))
	if err != nil {
		conn.Do("DISCARD")
		return
	}

	reply, err := conn.Do("EXEC")
	if err != nil {
		return
	}
	bulkReply, err := redis.Values(reply, err)
	if err != nil {
		return
	}
	if len(bulkReply) != 2 {
		return
	}

	msgObjs, err := redis.Values(bulkReply[0], nil)
	if err != nil {
		return
	}
	msgIds, err := redis.Values(bulkReply[1], nil)
	if err != nil {
		return
	}
	n := len(msgObjs)
	if n == 0 {
		return
	}
	msgShadow := make([]*rpc.MessageContainer, 0, n)
	removed := make([]interface{}, 1, n+1)
	removed[0] = msgQK

	for i, reply := range msgObjs {
		var data []byte
		var msg *rpc.MessageContainer
		if reply == nil {
			id, err := redis.String(msgIds[i], nil)
			if err == nil {
				removed = append(removed, id)
			}
			continue
		}
		data, err = redis.Bytes(reply, err)
		if err != nil {
			return
		}
		if len(data) == 0 {
			id, err := redis.String(msgIds[i], nil)
			if err == nil {
				removed = append(removed, id)
			}
			continue
		}
		msg, err = msgUnmarshal(data)

		if since.Before(msg.Birthday) {
			msgShadow = append(msgShadow, msg)
		}
	}

	if len(removed) > 1 {
		_, err = conn.Do("SREM", removed...)
		if err != nil {
			return
		}
	}
	msgs = msgShadow
	return
}
