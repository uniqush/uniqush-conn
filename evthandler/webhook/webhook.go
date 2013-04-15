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

package webhook

import (
	"net/http"
	"encoding/json"
	"bytes"
)

type webHook struct {
	url string
}

func (self *webHook) post(data interface{}) int {
	if len(self.url) == 0 || self.url == "none" {
		return 404
	}
	jdata, err := json.Marshal(data)
	if err != nil {
		return 404
	}
	resp, err := http.Post(self.url, "application/json", bytes.NewReader(jdata))
	if err != nil {
		return 404
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

type loginEvent struct {
	Service string `json:"service"`
	Username string `json:"username"`
	ConnID string `json:"connId"`
}

type LoginHandler struct {
	webHook
}

func (self *LoginHandler) OnLogin(service, username, connId string) {
	self.post(&loginEvent{service, username, connId})
}

type logoutEvent struct {
	Service string `json:"service"`
	Username string `json:"username"`
	ConnID string `json:"connId"`
	Reason string `json:"reason"`
}

type LogoutHandler struct {
	webHook
}

func (self *LogoutHandler) OnLogout(service, username, connId string, reason error) {
	self.post(&logoutEvent{service, username, connId, reason.Error()})
}


