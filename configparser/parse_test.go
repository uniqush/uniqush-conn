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

package configparser

import (
	"testing"
	"os"
)

func writeConfigFile(filename string) {
	config := `
uniqush_push: http://localhost:9898
default:
  timeout: 3s
  msg: http://localhost:8080/msg
  fwd: http://localhost:8080/fwd
  err: http://localhost:8080/err
  login: http://localhost:8080/login
  logout: http://localhost:8080/logout
  max_conns: 2048
  max_online_users: 2048
  max_conns_per_user: 10
  db:
    engine: redis
    addr: 127.0.0.1:6379
    name: 1
`
	file, _ := os.Create(filename)
	file.WriteString(config)
	file.Close()
}

func deleteConfigFile(filename string) {
	os.Remove(filename)
}

func TestParse(t *testing.T) {
	filename := "config.yaml"
	writeConfigFile(filename)
	defer deleteConfigFile(filename)
	_, err := Parse(filename)
	if err != nil {
		t.Errorf("Error: %v\n", err)
	}
}

