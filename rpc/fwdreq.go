/*
 * Copyright 2012 Nan Deng
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

package rpc

import "time"

type ForwardRequest struct {
	DontAsk         bool          `json:"dont-ask-permission,omitempty"`
	DontPush        bool          `json:"dont-push,omitempty"`
	Receiver        string        `json:"receiver"`
	ReceiverService string        `json:"receiver-service"`
	TTL             time.Duration `json:"ttl"`
	MessageContainer
}
