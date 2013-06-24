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
	"github.com/uniqush/uniqush-conn/proto"
	"time"
)

type Cache interface {
	CacheMessage(service, username string, msg *proto.Message, ttl time.Duration) (id string, err error)
	GetThenDel(service, username, id string) (msg *proto.Message, err error)

	// XXX Is there any better way to support retrieve all feature?
	Get(service, username, id string) (msg *proto.Message, err error)
	Del(service, username, id string) error
	GetCachedMessages(service, username string, excludes ...string) (msgs []*proto.Message, err error)
}
