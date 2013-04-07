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

package proto

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"
)

type opBetweenWriteAndRead interface {
	Op()
}

func testSendingCommands(t *testing.T, op opBetweenWriteAndRead, compress, encrypt bool, from, to *commandIO, cmds ...*Command) {
	errCh := make(chan error)
	startRead := make(chan bool)
	go func() {
		defer close(errCh)
		if op != nil {
			<-startRead
		}
		for i, cmd := range cmds {
			recved, err := to.ReadCommand()
			if err != nil {
				errCh <- err
				return
			}
			if !cmd.eq(recved) {
				errCh <- fmt.Errorf("%vth command does not equal", i)
			}
		}
	}()

	for _, cmd := range cmds {
		err := from.WriteCommand(cmd, compress, encrypt)
		if err != nil {
			t.Errorf("Error on write: %v", err)
		}
	}
	if op != nil {
		op.Op()
		startRead <- true
	}

	for err := range errCh {
		if err != nil {
			t.Errorf("Error on read: %v", err)
		}
	}
}

func getBufferCommandIOs(t *testing.T) (io1, io2 *commandIO, buffer *bytes.Buffer, ks *keySet) {
	keybuf := make([]byte, 2*(authKeyLen+encrKeyLen))
	io.ReadFull(rand.Reader, keybuf)
	sen := keybuf[:encrKeyLen]
	keybuf = keybuf[encrKeyLen:]
	sau := keybuf[:authKeyLen]
	keybuf = keybuf[authKeyLen:]
	cen := keybuf[:encrKeyLen]
	keybuf = keybuf[encrKeyLen:]
	cau := keybuf[:authKeyLen]
	keybuf = keybuf[authKeyLen:]

	buffer = new(bytes.Buffer)
	ks = newKeySet(sen, sau, cen, cau)
	scmdio := ks.getServerCommandIO(buffer)
	ccmdio := ks.getClientCommandIO(buffer)
	io1 = scmdio
	io2 = ccmdio
	return
}

func getNetworkCommandIOs(t *testing.T) (io1, io2 *commandIO) {
	sks, cks, s2c, c2s := exchangeKeysOrReport(t, true)
	if sks == nil || cks == nil || s2c == nil || c2s == nil {
		return
	}

	scmdio := sks.getServerCommandIO(s2c)
	ccmdio := cks.getClientCommandIO(c2s)
	io1 = scmdio
	io2 = ccmdio
	return
}

func TestExchangingFullCommandNoCompressNoEncrypt(t *testing.T) {
	cmd := randomCommand()
	compress := false
	encrypt := false
	io1, io2 := getNetworkCommandIOs(t)
	testSendingCommands(t, nil, compress, encrypt, io1, io2, cmd)
	testSendingCommands(t, nil, compress, encrypt, io2, io1, cmd)
}

func TestExchangingFullCommandNoCompress(t *testing.T) {
	cmd := randomCommand()
	compress := false
	encrypt := true
	io1, io2 := getNetworkCommandIOs(t)
	testSendingCommands(t, nil, compress, encrypt, io1, io2, cmd)
	testSendingCommands(t, nil, compress, encrypt, io2, io1, cmd)
}

func TestExchangingFullCommandNoEncrypt(t *testing.T) {
	cmd := randomCommand()
	compress := true
	encrypt := false
	io1, io2 := getNetworkCommandIOs(t)
	testSendingCommands(t, nil, compress, encrypt, io1, io2, cmd)
	testSendingCommands(t, nil, compress, encrypt, io2, io1, cmd)
}

type bufPrinter struct {
	buf     *bytes.Buffer
	authKey []byte
}

func (self *bufPrinter) Op() {
	fmt.Printf("--------------\n")
	fmt.Printf("Data in buffer: %v\n", self.buf.Bytes())

	data := self.buf.Bytes()
	data = data[16:]

	hash := hmac.New(sha256.New, self.authKey)
	hash.Write(data)

	fmt.Printf("HMAC: %v\n", hash.Sum(nil))
	fmt.Printf("--------------\n")
}

func TestExchangingFullCommandOverNetwork(t *testing.T) {
	cmd := randomCommand()
	compress := true
	encrypt := true
	io1, io2 := getNetworkCommandIOs(t)
	testSendingCommands(t, nil, compress, encrypt, io1, io2, cmd)
	testSendingCommands(t, nil, compress, encrypt, io2, io1, cmd)
}

func TestExchangingFullCommandInBuffer(t *testing.T) {
	cmd := randomCommand()
	compress := true
	encrypt := true
	io1, io2 := getNetworkCommandIOs(t)
	io1, io2, buffer, ks := getBufferCommandIOs(t)
	op := &bufPrinter{buffer, ks.serverAuthKey}
	testSendingCommands(t, op, compress, encrypt, io1, io2, cmd)

	op = &bufPrinter{buffer, ks.clientAuthKey}
	testSendingCommands(t, op, compress, encrypt, io2, io1, cmd)
}

func randomCommand() *Command {
	cmd := new(Command)
	cmd.Type = 1
	cmd.Params = make([]string, 2)
	cmd.Params[0] = "123"
	cmd.Params[1] = "223"
	cmd.Message = new(Message)
	cmd.Message.Header = make(map[string]string, 2)
	cmd.Message.Header["a"] = "hello"
	cmd.Message.Header["b"] = "hell"
	cmd.Message.Body = make([]byte, 10)
	io.ReadFull(rand.Reader, cmd.Message.Body)
	return cmd
}

func TestExchangingMultiFullCommandOverNetwork(t *testing.T) {
	cmds := make([]*Command, 100)
	for i, _ := range cmds {
		cmd := randomCommand()
		cmds[i] = cmd
	}

	compress := true
	encrypt := true
	io1, io2 := getNetworkCommandIOs(t)
	testSendingCommands(t, nil, compress, encrypt, io1, io2, cmds...)
	testSendingCommands(t, nil, compress, encrypt, io2, io1, cmds...)
}

func TestConcurrentWrite(t *testing.T) {
	N := 100
	cmds := make([]*Command, N)
	for i, _ := range cmds {
		cmd := randomCommand()
		cmds[i] = cmd
	}
	io1, io2 := getNetworkCommandIOs(nil)
	done := make(chan bool)
	go func() {
		defer close(done)
		for i := 0; i < N; i++ {
			io2.ReadCommand()
		}
	}()

	for _, cmd := range cmds {
		go io1.WriteCommand(cmd, true, true)
	}
	<-done
}

func BenchmarkExchangingMultiFullCommandOverNetwork(b *testing.B) {
	b.StopTimer()
	cmds := make([]*Command, b.N)
	for i, _ := range cmds {
		cmd := randomCommand()
		cmds[i] = cmd
	}
	io1, io2 := getNetworkCommandIOs(nil)
	done := make(chan bool)
	go func() {
		defer close(done)
		for i := 0; i < b.N; i++ {
			io2.ReadCommand()
		}
	}()

	b.StartTimer()
	for _, cmd := range cmds {
		io1.WriteCommand(cmd, true, true)
	}
	<-done
}

func BenchmarkExchangingMultiFullCommandNoEncrypt(b *testing.B) {
	b.StopTimer()
	cmds := make([]*Command, b.N)
	for i, _ := range cmds {
		cmd := randomCommand()
		cmds[i] = cmd
	}
	io1, io2 := getNetworkCommandIOs(nil)
	done := make(chan bool)
	go func() {
		defer close(done)
		for i := 0; i < b.N; i++ {
			io2.ReadCommand()
		}
	}()

	b.StartTimer()
	for _, cmd := range cmds {
		io1.WriteCommand(cmd, true, false)
	}
	<-done
}

func BenchmarkExchangingMultiFullCommandNoEncryptNoCompress(b *testing.B) {
	b.StopTimer()
	cmds := make([]*Command, b.N)
	for i, _ := range cmds {
		cmd := randomCommand()
		cmds[i] = cmd
	}
	io1, io2 := getNetworkCommandIOs(nil)
	done := make(chan bool)
	go func() {
		defer close(done)
		for i := 0; i < b.N; i++ {
			io2.ReadCommand()
		}
	}()

	b.StartTimer()
	for _, cmd := range cmds {
		io1.WriteCommand(cmd, false, false)
	}
	<-done
}
