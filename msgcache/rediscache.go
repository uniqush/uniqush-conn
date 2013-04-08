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
	"github.com/garyburd/redigo/redis"
	"github.com/uniqush/uniqush-conn/proto"
	"time"
	"fmt"
	"encoding/json"
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
		MaxIdle: 3,
		IdleTimeout: 240 * time.Second,
		Dial: dial,
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

func marshalMsg(msg *proto.Message) (buf []byte, err error) {
	buf, err = json.Marshal(msg)
	return
}

func unmarshalMsg(buf []byte) (msg *proto.Message, err error) {
	msg = new(proto.Message)
	err = json.Unmarshal(buf, msg)
	if err != nil {
		msg = nil
	}
	return
}

func (self *redisMessageCache) Enqueue(service, username string, msg *proto.Message) (id int, err error) {
	key := mqKey(service, username)
	value, err := marshalMsg(msg)
	if err != nil {
		return
	}
	conn := self.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("RPUSH", key, value)
	id, err = redis.Int(reply, err)
	return
}

func (self *redisMessageCache) Dequeue(service, username string) (msg *proto.Message, err error) {
	key := mqKey(service, username)
	if err != nil {
		return
	}
	conn := self.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("LPOP", key)
	if reply == nil {
		msg = nil
		err = nil
		return
	}
	value, err := redis.Bytes(reply, err)
	if err != nil {
		return
	}
	if value == nil {
		msg = nil
		return
	}
	msg, err = unmarshalMsg(value)
	return
}

