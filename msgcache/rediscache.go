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
	"github.com/uniqush/uniqush-conn/proto"
	"strconv"
	"time"
)

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

func mqKey(service, username string) string {
	return fmt.Sprintf("mq:%v:%v", service, username)
}

func mboxKey(service, username string) string {
	return fmt.Sprintf("mbox:%v:%v", service, username)
}

func posterKey(service, username string) string {
	return fmt.Sprintf("poster:%v:%v", service, username)
}

type msgContainer struct {
	Id  int64
	Msg *proto.Message
}

func marshalMsg(msg *proto.Message, id int64) (buf []byte, err error) {
	mc := new(msgContainer)
	mc.Id = id
	mc.Msg = msg
	buf, err = json.Marshal(mc)
	return
}

func unmarshalMsg(buf []byte) (msg *proto.Message, err error) {
	mc := new(msgContainer)
	err = json.Unmarshal(buf, &mc)
	if err != nil {
		return
	}
	msg = mc.Msg
	return
}

func (self *redisMessageCache) Enqueue(service, username string, msg *proto.Message) (id string, err error) {
	key := mqKey(service, username)
	score := time.Now().UnixNano()
	value, err := marshalMsg(msg, score)
	if err != nil {
		return
	}
	conn := self.pool.Get()
	defer conn.Close()
	_, err = conn.Do("ZADD", key, score, value)
	if err != nil {
		return
	}
	id = fmt.Sprintf("%v", score)
	return
}

func (self *redisMessageCache) dequeueN(service, username string, n int) (msgs []*proto.Message, err error) {
	key := mqKey(service, username)
	if err != nil {
		return
	}
	conn := self.pool.Get()
	defer conn.Close()

	err = conn.Send("MULTI")
	if err != nil {
		return
	}
	err = conn.Send("ZRANGE", key, 0, n-1)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	err = conn.Send("ZREMRANGEBYRANK", key, 0, n-1)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	mbulk, err := conn.Do("EXEC")

	if err != nil {
		return
	}
	if mbulk == nil {
		return
	}
	mreply, err := redis.Values(mbulk, err)
	if err != nil {
		return
	}
	if len(mreply) < 2 {
		return
	}
	reply := mreply[0]
	values, err := redis.Values(reply, err)
	if err != nil {
		return
	}
	if len(values) == 0 {
		return
	}

	msgs = make([]*proto.Message, len(values))

	for i, r := range values {
		buf, e := redis.Bytes(r, nil)
		if e != nil {
			err = e
			msgs = nil
			return
		}
		msg, e := unmarshalMsg(buf)
		if e != nil {
			err = e
			msgs = nil
			return
		}
		msgs[i] = msg
	}
	return
}

func (self *redisMessageCache) Dequeue(service, username string) (msg *proto.Message, err error) {
	msgs, err := self.dequeueN(service, username, 1)
	if err != nil {
		return
	}
	if len(msgs) == 0 {
		msg = nil
		return
	}
	msg = msgs[0]
	return
}

func (self *redisMessageCache) Clrqueue(service, username string) (msgs []*proto.Message, err error) {
	return self.dequeueN(service, username, 0)
}

func (self *redisMessageCache) DelFromQueue(service, username, id string) (msg *proto.Message, err error) {
	score, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return
	}
	key := mqKey(service, username)
	conn := self.pool.Get()
	defer conn.Close()

	err = conn.Send("MULTI")
	if err != nil {
		return
	}
	err = conn.Send("ZRANGEBYSCORE", key, score, score)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	err = conn.Send("ZREMRANGEBYSCORE", key, score, score)
	if err != nil {
		conn.Do("DISCARD")
		return
	}
	reply, err := conn.Do("EXEC")
	if err != nil {
		return
	}
	if reply == nil {
		msg = nil
		err = nil
		return
	}
	values, err := redis.Values(reply, err)
	if err != nil {
		return
	}
	if len(values) < 2 {
		return
	}

	rangeReply := values[0]
	if rangeReply == nil {
		return
	}

	msgs, err := redis.Values(rangeReply, err)
	if len(msgs) == 0 {
		return
	}

	value, err := redis.Bytes(msgs[0], nil)
	if err != nil {
		return
	}
	if value == nil {
		return
	}
	msg, err = unmarshalMsg(value)
	return
}

func (self *redisMessageCache) SetMessageBox(service, username string, msg *proto.Message, timeout time.Duration) error {
	key := mboxKey(service, username)
	value, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	conn := self.pool.Get()
	defer conn.Close()

	if timeout.Seconds() <= 0.0 {
		_, err = conn.Do("SET", key, value)
	} else {
		_, err = conn.Do("SETEX", key, int64(timeout.Seconds()), value)
	}
	if err != nil {
		return err
	}
	return nil
}

func (self *redisMessageCache) GetMessageBox(service, username string) (msg *proto.Message, err error) {
	key := mboxKey(service, username)
	conn := self.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("GET", key)
	if err != nil {
		return
	}
	if reply == nil {
		return
	}

	value, err := redis.Bytes(reply, err)
	if err != nil {
		return
	}
	msg = new(proto.Message)
	err = json.Unmarshal(value, msg)
	if err != nil {
		msg = nil
		return
	}
	return
}

