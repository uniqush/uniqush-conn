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
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uniqush/uniqush-conn/rpc"
	"time"
)

const (
	maxMessageIdLength   = 255
	maxUsernameLength    = 255
	maxServicenameLength = 255
)

type mysqlCacheManager struct {
}

func (self *mysqlCacheManager) Engine() string {
	return "mysql"
}

func (self *mysqlCacheManager) GetCache(addr, username, password, database string) (Cache, error) {
	return NewMySQLMessageCache(username, password, addr, database)
}

type mysqlMessageCache struct {
	db               *sql.DB
	cacheStmt        *sql.Stmt
	getMultiMsgStmt  *sql.Stmt
	getSingleMsgStmt *sql.Stmt
}

func NewMySQLMessageCache(username, password, address, dbname string) (c *mysqlMessageCache, err error) {
	if len(address) == 0 {
		address = "127.0.0.1:3306"
	}
	dsn := fmt.Sprintf("%v:%v@tcp(%v)/%v", username, password, address, dbname)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return
	}

	stmt, err := db.Prepare(`INSERT INTO messages
		(mid, owner_service, owner_name, sender_service, sender_name, create_time, deadline, content)
		VALUES
		(?, ?, ?, ?, ?, ?, ?, ?)
		`)
	if err != nil {
		return
	}

	c = new(mysqlMessageCache)
	c.cacheStmt = stmt
	c.db = db

	stmt, err = db.Prepare(`SELECT mid, sender_service, sender_name, content
		FROM messages
		WHERE owner_service=? AND owner_name=? AND create_time>=? AND (deadline>=? OR deadline<=0) ORDER BY create_time;
		`)
	if err != nil {
		return
	}
	c.getMultiMsgStmt = stmt
	stmt, err = db.Prepare(`SELECT mid, sender_service, sender_name, content
		FROM messages
		WHERE mid=? AND (deadline>? OR deadline<=0);
		`)
	if err != nil {
		return
	}
	c.getSingleMsgStmt = stmt
	return
}

func (self *mysqlMessageCache) CacheMessage(service, username string, mc *rpc.MessageContainer, ttl time.Duration) (id string, err error) {
	data, err := json.Marshal(mc.Message)
	if err != nil {
		return
	}

	id = randomId()
	if len(id) > maxMessageIdLength {
		err = fmt.Errorf("message id length is greater than %v characters", maxMessageIdLength)
		return
	}

	if len(username) > maxUsernameLength {
		err = fmt.Errorf("user %v's name is too long", username)
		return
	}
	if len(mc.Sender) > maxUsernameLength {
		err = fmt.Errorf("user %v's name is too long", mc.Sender)
		return
	}
	if len(service) > maxServicenameLength {
		err = fmt.Errorf("service %v's name is too long", service)
		return
	}
	if len(mc.SenderService) > maxServicenameLength {
		err = fmt.Errorf("service %v's name is too long", mc.SenderService)
		return
	}
	mc.Id = id

	now := time.Now()
	deadline := now.Add(ttl)
	if ttl < 1*time.Second {
		// max possible value for int64
		deadline = time.Unix(0, 0)
	}

	result, err := self.cacheStmt.Exec(id, service, username, mc.SenderService, mc.Sender, now.Unix(), deadline.Unix(), data)
	if err != nil {
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		return
	}
	if n != 1 {
		err = fmt.Errorf("affected %v rows, which is weird", n)
		return
	}
	return
}

func (self *mysqlMessageCache) Get(service, username, id string) (mc *rpc.MessageContainer, err error) {
	row := self.getSingleMsgStmt.QueryRow(id, time.Now().Unix())
	if err != nil {
		return
	}

	mc = new(rpc.MessageContainer)
	var data []byte
	err = row.Scan(&mc.Id, &mc.SenderService, &mc.Sender, &data)
	if err != nil {
		if err == sql.ErrNoRows {
			err = nil
			mc = nil
			return
		}
		return
	}
	mc.Message = new(rpc.Message)
	err = json.Unmarshal(data, mc.Message)
	if err != nil {
		return
	}
	return
}

func (self *mysqlMessageCache) RetrieveAllSince(service, username string, since time.Time) (msgs []*rpc.MessageContainer, err error) {
	rows, err := self.getMultiMsgStmt.Query(service, username, since.Unix(), time.Now().Unix())
	if err != nil {
		return
	}
	defer rows.Close()

	msgs = make([]*rpc.MessageContainer, 0, 128)
	for rows.Next() {
		mc := new(rpc.MessageContainer)
		var data []byte
		err = rows.Scan(&mc.Id, &mc.SenderService, &mc.Sender, &data)
		if err != nil {
			return
		}
		mc.Message = new(rpc.Message)
		err = json.Unmarshal(data, mc.Message)
		if err != nil {
			return
		}
		msgs = append(msgs, mc)
	}
	return
}
