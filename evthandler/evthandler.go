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

package evthandler

import (
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/server"
	"time"
)

type LoginHandler interface {
	OnLogin(service, username, connId string)
}

type LogoutHandler interface {
	OnLogout(service, username, connId string, reason error)
}

type MessageHandler interface {
	OnMessage(connId string, msg *proto.Message)
}

type ForwardRequestHandler interface {
	ShouldForward(fwd *server.ForwardRequest) bool
	MaxTTL() time.Duration
}

type ErrorHandler interface {
	OnError(service, username, connId, addr string, err error)
}

type SubscribeHandler interface {
	ShouldSubscribe(service, username string, info map[string]string) bool
}

type UnsubscribeHandler interface {
	OnUnsubscribe(service, username string, info map[string]string)
}

type PushHandler interface {
	ShouldPush(service, username string, info map[string]string) bool
}
