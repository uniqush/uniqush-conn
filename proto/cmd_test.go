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

package proto

import (
	"fmt"
	"github.com/uniqush/uniqush-conn/rpc"
	//"labix.org/v2/mgo/bson"
	"testing"
)

func marshalUnmarshal(cmd *Command) error {
	data, err := cmd.Marshal()
	if err != nil {
		return err
	}

	c, err := UnmarshalCommand(data)
	if !c.eq(cmd) {
		return fmt.Errorf("Not same")
	}
	return nil
}

func TestCommandMarshalNoParamsNoMessage(t *testing.T) {
	cmd := new(Command)
	cmd.Type = 1
	marshalUnmarshal(cmd)
}

func TestCommandMarshalNoParams(t *testing.T) {
	cmd := new(Command)
	cmd.Type = 1
	cmd.Params = make([]string, 2)
	cmd.Params[0] = "hello"
	cmd.Params[1] = ""
	marshalUnmarshal(cmd)
}

func TestCommandMarshalNoBody(t *testing.T) {
	cmd := new(Command)
	cmd.Type = 1
	cmd.Params = make([]string, 2)
	cmd.Params[0] = "hello"
	cmd.Params[1] = ""
	cmd.Message = new(rpc.Message)
	cmd.Message.Header = make(map[string]string, 3)
	cmd.Message.Header["a"] = "h"
	cmd.Message.Header["b"] = "i"
	cmd.Message.Header["b"] = "j"
	marshalUnmarshal(cmd)
}

func TestCommandMarshal(t *testing.T) {
	cmd := new(Command)
	cmd.Type = 1
	cmd.Params = make([]string, 2)
	cmd.Params[0] = "hello"
	cmd.Params[1] = "new"
	cmd.Message = new(rpc.Message)
	cmd.Message.Header = make(map[string]string, 3)
	cmd.Message.Header["a"] = "h"
	cmd.Message.Header["b"] = "i"
	cmd.Message.Header["b"] = "j"
	cmd.Message.Body = []byte{1, 2, 3, 3}
	marshalUnmarshal(cmd)
}

func TestRandomize(t *testing.T) {
	cmd := new(Command)
	cmd.Type = 1
	cmd.Params = make([]string, 2)
	cmd.Params[0] = "hello"
	cmd.Params[1] = "new"
	cmd.Message = new(rpc.Message)
	cmd.Message.Header = make(map[string]string, 3)
	cmd.Message.Header["a"] = "h"
	cmd.Message.Header["b"] = "i"
	cmd.Message.Header["b"] = "j"
	cmd.Message.Body = []byte{1, 2, 3, 3}
	marshalUnmarshal(cmd)
	cmd.Randomize()
}

func BenchmarkCommandMarshalUnmarshal(b *testing.B) {
	b.StopTimer()
	cmds := make([]*Command, b.N)
	for i, _ := range cmds {
		cmd := randomCommand()
		cmds[i] = cmd
	}
	b.StartTimer()
	for _, cmd := range cmds {
		data, _ := cmd.Marshal()
		UnmarshalCommand(data)
	}
}

// We don't need this benchmark to run for all ci test.
// func BenchmarkCommandMarshalUnmarshalBson(b *testing.B) {
// 	b.StopTimer()
// 	cmds := make([]*Command, b.N)
// 	for i, _ := range cmds {
// 		cmd := randomCommand()
// 		cmds[i] = cmd
// 	}
// 	b.StartTimer()
// 	for _, cmd := range cmds {
// 		data, _ := bson.Marshal(cmd)
// 		c := new(Command)
// 		bson.Unmarshal(data, c)
// 	}
// }
