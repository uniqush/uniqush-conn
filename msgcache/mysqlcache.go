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
	"net"
	"sync"
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

func (self *mysqlCacheManager) GetCache(host, username, password, database string, port int) (Cache, error) {
	if len(host) == 0 {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 3306
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%v", port))
	return newMySQLMessageCache(username, password, addr, database)
}

type mysqlMessageCache struct {
	dsn              string
	lock             sync.RWMutex
	db               *sql.DB
	cacheStmt        *sql.Stmt
	getMultiMsgStmt  *sql.Stmt
	getSingleMsgStmt *sql.Stmt
}

func (self *mysqlMessageCache) init() error {
	createTblStmt := `CREATE TABLE IF NOT EXISTS messages
	(
		id CHAR(255) NOT NULL PRIMARY KEY,
		mid CHAR(255),

		owner_service CHAR(255) NOT NULL,
		owner_name CHAR(255) NOT NULL,

		sender_service CHAR(255),
		sender_name CHAR(255),

		create_time BIGINT,
		deadline BIGINT,
		content BLOB
	);`
	createIdxStmt := `CREATE INDEX idx_owner_time ON messages (owner_service, owner_name, create_time, deadline);`

	tx, err := self.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(createTblStmt)
	if err != nil {
		return err
	}
	// XXX we will ignore the error.
	// Because it will always be an error if there exists
	// an index with same name.
	// Is there any way to only create index if it does not exist?
	tx.Exec(createIdxStmt)
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (self *mysqlMessageCache) Close() error {
	self.lock.RLock()
	defer self.lock.RUnlock()
	if self.cacheStmt != nil {
		self.cacheStmt.Close()
	}
	if self.getMultiMsgStmt != nil {
		self.getMultiMsgStmt.Close()
	}
	if self.getSingleMsgStmt != nil {
		self.getSingleMsgStmt.Close()
	}
	if self.db != nil {
		self.db.Close()
	}
	return nil
}

func (self *mysqlMessageCache) reconnect() error {
	self.Close()
	self.lock.Lock()
	defer self.lock.Unlock()
	if len(self.dsn) == 0 {
		return fmt.Errorf("No DSN")
	}
	db, err := sql.Open("mysql", self.dsn)
	if err != nil {
		return fmt.Errorf("Data base error: %v", err)
	}
	self.db = db
	err = self.init()
	if err != nil {
		return fmt.Errorf("Data base init error: %v", err)
	}

	stmt, err := db.Prepare(`INSERT INTO messages
		(id, mid, owner_service, owner_name, sender_service, sender_name, create_time, deadline, content)
		VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	if err != nil {
		return fmt.Errorf("Data base prepare statement error: %v; insert stmt", err)
	}

	self.cacheStmt = stmt

	stmt, err = db.Prepare(`SELECT mid, sender_service, sender_name, create_time, content
		FROM messages
		WHERE owner_service=? AND owner_name=? AND create_time>=? AND (deadline>=? OR deadline<=0) ORDER BY create_time;
		`)
	if err != nil {
		return fmt.Errorf("Data base prepare error: %v; select multi stmt", err)
	}
	self.getMultiMsgStmt = stmt
	stmt, err = db.Prepare(`SELECT mid, sender_service, sender_name, create_time, content
		FROM messages
		WHERE id=? AND (deadline>? OR deadline<=0);
		`)
	if err != nil {
		return fmt.Errorf("Data base prepare error: %v; select single stmt", err)
	}
	self.getSingleMsgStmt = stmt
	return nil
}

func newMySQLMessageCache(username, password, address, dbname string) (c *mysqlMessageCache, err error) {
	c = new(mysqlMessageCache)
	if len(address) == 0 {
		address = "127.0.0.1:3306"
	}
	c.dsn = fmt.Sprintf("%v:%v@tcp(%v)/%v", username, password, address, dbname)
	err = c.reconnect()
	return
}

func getUniqMessageId(service, username, id string) string {
	return fmt.Sprintf("%v,%v,%v", service, username, id)
}

func (self *mysqlMessageCache) CacheMessage(service, username string, mc *rpc.MessageContainer, ttl time.Duration) (id string, err error) {
	data, err := json.Marshal(mc.Message)
	if err != nil {
		return
	}

	if mc.Id == "" {
		id = randomId()
		mc.Id = id
	} else {
		id = mc.Id
	}

	uniqid := getUniqMessageId(service, username, id)
	if len(uniqid) > maxMessageIdLength {
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

	now := time.Now()
	mc.Birthday = now
	deadline := now.Add(ttl)
	if ttl < 1*time.Second {
		// max possible value for int64
		deadline = time.Unix(0, 0)
	}
	self.lock.RLock()
	defer self.lock.RUnlock()

	result, err := self.cacheStmt.Exec(uniqid, id, service, username, mc.SenderService, mc.Sender, now.Unix(), deadline.Unix(), data)
	if err != nil {
		err = fmt.Errorf("Data base error: %v; insert error", err)
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
	self.lock.RLock()
	defer self.lock.RUnlock()
	uniqid := getUniqMessageId(service, username, id)
	row := self.getSingleMsgStmt.QueryRow(uniqid, time.Now().Unix())
	if err != nil {
		err = fmt.Errorf("Data base error: %v; query error", err)
		return
	}

	mc = new(rpc.MessageContainer)
	var data []byte
	var createTime int64
	err = row.Scan(&mc.Id, &mc.SenderService, &mc.Sender, &createTime, &data)
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
	mc.Birthday = time.Unix(createTime, 0)
	return
}

func (self *mysqlMessageCache) RetrieveAllSince(service, username string, since time.Time) (msgs []*rpc.MessageContainer, err error) {
	self.lock.RLock()
	defer self.lock.RUnlock()
	rows, err := self.getMultiMsgStmt.Query(service, username, since.Unix(), time.Now().Unix())
	if err != nil {
		err = fmt.Errorf("Data base error: %v; query multi-msg error", err)
		return
	}
	defer rows.Close()

	msgs = make([]*rpc.MessageContainer, 0, 128)
	for rows.Next() {
		mc := new(rpc.MessageContainer)
		var data []byte
		var createTime int64
		err = rows.Scan(&mc.Id, &mc.SenderService, &mc.Sender, &createTime, &data)
		if err != nil {
			return
		}
		mc.Message = new(rpc.Message)
		err = json.Unmarshal(data, mc.Message)
		if err != nil {
			return
		}
		mc.Birthday = time.Unix(createTime, 0)
		msgs = append(msgs, mc)
	}
	return
}
