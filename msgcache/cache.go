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
	SetMail(service, username string, msg *proto.Message, ttl time.Duration) (id string, err error)
	SetPoster(service, username, key string, msg *proto.Message, ttl time.Duration) (id string, err error)

	// Mail can only be read once before expire
	// Poster can be read many times before expire
	GetOrDel(service, username, id string) (msg *proto.Message, err error)
}
