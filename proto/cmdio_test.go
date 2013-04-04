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
	"testing"
	"fmt"
	"bytes"
	"io"
	"crypto/rand"
	"crypto/sha256"
	"crypto/hmac"
)

type opBetweenWriteAndRead interface {
	Op()
}

func testSendingCommands(t *testing.T, op opBetweenWriteAndRead, from, to *commandIO, cmds ...*command) {
	errCh := make(chan error)
	startRead := make(chan bool)
	go func() {
		defer close(errCh)
		if op != nil {
			<-startRead
		}
		for i, cmd := range cmds {
			fmt.Printf("Reading command...\n")
			recved, err := to.ReadCommand()
			if err != nil {
				errCh <- err
				return
			}
			if !cmd.eq(recved) {
				errCh <- fmt.Errorf("%vth command does not equal", i)
			}
		}
		fmt.Printf("Read Done\n")
	}()

	for _, cmd := range cmds {
		fmt.Printf("Writing command...\n")
		err := from.WriteCommand(cmd)
		if err != nil {
			t.Errorf("Error on write: %v", err)
		}
	}
	fmt.Printf("Write Done\n")
	if op != nil {
		op.Op()
		startRead<-true
	}

	for err := range errCh {
		if err != nil {
			t.Errorf("Error on read: %v", err)
		}
	}
}

func getBufferCommandIOs(t *testing.T, compress, encrypt bool) (io1, io2 *commandIO, buffer *bytes.Buffer, ks *keySet) {
	keybuf := make([]byte, 2 * (authKeyLen + encrKeyLen))
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
	scmdio = confCommandIO(scmdio, compress, encrypt)
	ccmdio := ks.getClientCommandIO(buffer)
	ccmdio = confCommandIO(ccmdio, compress, encrypt)
	io1 = scmdio
	io2 = ccmdio
	return
}

func confCommandIO(cmdio *commandIO, compress, encrypt bool) *commandIO {
	if !compress {
		cmdio.ReadCompressOff()
		cmdio.WriteCompressOff()
	}
	if !encrypt {
		cmdio.ReadEncryptOff()
		cmdio.WriteEncryptOff()
	}
	return cmdio
}

func getNetworkCommandIOs(t *testing.T, compress, encrypt bool) (io1, io2 *commandIO) {
	sks, cks, s2c, c2s := exchangeKeysOrReport(t)
	if sks == nil || cks == nil || s2c == nil || c2s == nil {
		return
	}

	scmdio := sks.getServerCommandIO(s2c)
	scmdio = confCommandIO(scmdio, compress, encrypt)
	ccmdio := cks.getClientCommandIO(c2s)
	ccmdio = confCommandIO(ccmdio, compress, encrypt)
	io1 = scmdio
	io2 = ccmdio
	return
}

func TestExchangingFullCommandNoCompressNoEncrypt(t *testing.T) {
	cmd := new(command)
	cmd.Body = []byte{1,2,3}
	cmd.Type = 1
	cmd.Params = make([][]byte, 2)
	cmd.Params[0] = []byte{1,2,3}
	cmd.Params[1] = []byte{2,2,3}
	cmd.Header = make(map[string]string, 2)
	cmd.Header["a"] = "hello"
	cmd.Header["b"] = "hell"
	io1, io2 := getNetworkCommandIOs(t, false, false)
	testSendingCommands(t, nil, io1, io2, cmd)
	testSendingCommands(t, nil, io2, io1, cmd)
}

func TestExchangingFullCommandNoEncrypt(t *testing.T) {
	cmd := new(command)
	cmd.Body = []byte{1,2,3}
	cmd.Type = 1
	cmd.Params = make([][]byte, 2)
	cmd.Params[0] = []byte{1,2,3}
	cmd.Params[1] = []byte{2,2,3}
	cmd.Header = make(map[string]string, 2)
	cmd.Header["a"] = "hello"
	cmd.Header["b"] = "hell"
	io1, io2 := getNetworkCommandIOs(t, true, false)
	testSendingCommands(t, nil, io1, io2, cmd)
	testSendingCommands(t, nil, io2, io1, cmd)
}

type bufPrinter struct {
	buf *bytes.Buffer
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

func TestExchangingFullCommand(t *testing.T) {
	fmt.Printf("TestExchangeFullCommand\n")
	cmd := new(command)
	cmd.Body = []byte{1,2,3}
	cmd.Type = 1
	cmd.Params = make([][]byte, 2)
	cmd.Params[0] = []byte{1,2,3}
	cmd.Params[1] = []byte{2,2,3}
	cmd.Header = make(map[string]string, 2)
	cmd.Header["a"] = "hello"
	cmd.Header["b"] = "hell"
	io1, io2, buffer, ks := getBufferCommandIOs(t, true, true)
	op := &bufPrinter{buffer, ks.serverAuthKey}
	testSendingCommands(t, op, io1, io2, cmd)

	op = &bufPrinter{buffer, ks.clientAuthKey}
	testSendingCommands(t, op, io2, io1, cmd)
}

