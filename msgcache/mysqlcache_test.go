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

type mysqlCachManager struct {
}

func (self *mysqlCachManager) Name() string {
	return "mysql"
}

func (self *mysqlCachManager) GetCache() (Cache, error) {
	return NewSQLMessageCache("uniqush", "uniqush-pass", "127.0.0.1:3306", "uniqush")
}

func (self *mysqlCachManager) ClearCache(c Cache) {
	if cache, ok := c.(*mysqlMessageCache); ok {
		cache.db.Exec("DELETE FROM messages")
	}
}

func TestMysqlCache(t *testing.T) {
	testCacheImpl(&mysqlCachManager{}, t)
}
