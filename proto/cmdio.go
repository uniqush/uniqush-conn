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
	"code.google.com/p/snappy-go/snappy"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
	"labix.org/v2/mgo/bson"
	"sync"
)

type commandIO struct {
	writeAuth   hash.Hash
	cryptWriter io.Writer
	readAuth    hash.Hash
	cryptReader io.Reader
	conn        io.ReadWriter

	writeLock *sync.Mutex
}

func (self *commandIO) writeThenHmac(data []byte, encrypt bool) (mac []byte, err error) {
	writer := self.cryptWriter
	if !encrypt {
		writer = self.conn
	} else {
		self.writeAuth.Reset()
	}
	err = writen(writer, data)
	if err != nil {
		return
	}
	if !encrypt {
		return
	}
	mac = self.writeAuth.Sum(nil)
	return
}

func (self *commandIO) readThenHmac(data []byte, encrypt bool) (mac []byte, err error) {
	reader := self.cryptReader
	if !encrypt {
		reader = self.conn
	} else {
		self.readAuth.Reset()
	}

	n, err := io.ReadFull(reader, data)
	if err != nil {
		return
	}
	if n != len(data) {
		err = io.EOF
		return
	}
	if !encrypt {
		return
	}
	mac = self.readAuth.Sum(nil)
	return
}

func (self *commandIO) writeHmac(mac []byte) error {
	if len(mac) == 0 {
		return nil
	}
	return writen(self.conn, mac)
}

func (self *commandIO) readAndCmpHmac(mac []byte) error {
	if len(mac) == 0 {
		return nil
	}
	macRecved := make([]byte, self.readAuth.Size())
	n, err := io.ReadFull(self.conn, macRecved)
	if err != nil {
		return err
	}
	if n != len(macRecved) {
		return ErrCorruptedData
	}
	if !bytesEq(mac, macRecved) {
		return ErrCorruptedData
	}
	return nil
}

func (self *commandIO) decodeCommand(data []byte, compress bool) (cmd *Command, err error) {
	decoded := data
	if !compress {
		decoded, err = snappy.Decode(nil, data)
		if err != nil {
			return
		}
	}
	cmd = new(Command)
	err = bson.Unmarshal(decoded, cmd)
	if err != nil {
		return
	}
	return
}

func (self *commandIO) encodeCommand(cmd *Command, compress bool) (data []byte, err error) {
	bsonEncoded, err := bson.Marshal(cmd)
	if err != nil {
		return
	}

	data = bsonEncoded
	if !compress {
		data, err = snappy.Encode(nil, bsonEncoded)
		if err != nil {
			return
		}
	}
	return
}

// WriteCommand() is goroutine-safe. i.e. Multiple goroutine could write concurrently.
func (self *commandIO) WriteCommand(cmd *Command, compress, encrypt bool) error {
	var flag uint16
	flag = 0
	if compress {
		flag |= cmdflag_COMPRESS
	}
	if encrypt {
		flag |= cmdflag_ENCRYPT
	}
	data, err := self.encodeCommand(cmd, compress)
	if err != nil {
		return err
	}
	var cmdLen uint16
	cmdLen = uint16(len(data))
	if cmdLen == 0 {
		return nil
	}
	self.writeLock.Lock()
	defer self.writeLock.Unlock()
	err = binary.Write(self.conn, binary.LittleEndian, cmdLen)
	if err != nil {
		return err
	}
	err = binary.Write(self.conn, binary.LittleEndian, flag)
	if err != nil {
		return err
	}
	mac, err := self.writeThenHmac(data, encrypt)
	if err != nil {
		return err
	}
	err = self.writeHmac(mac)
	if err != nil {
		return err
	}
	return nil
}

// ReadCommand() is not goroutine-safe.
func (self *commandIO) ReadCommand() (cmd *Command, err error) {
	var cmdLen uint16
	var flag uint16
	err = binary.Read(self.conn, binary.LittleEndian, &cmdLen)
	if err != nil {
		return
	}
	err = binary.Read(self.conn, binary.LittleEndian, &flag)
	if err != nil {
		return
	}

	compress := ((flag & cmdflag_COMPRESS) != 0)
	encrypt := ((flag & cmdflag_ENCRYPT) != 0)

	data := make([]byte, int(cmdLen))
	mac, err := self.readThenHmac(data, encrypt)
	if err != nil {
		return
	}
	err = self.readAndCmpHmac(mac)
	if err != nil {
		return
	}
	cmd, err = self.decodeCommand(data, compress)
	return
}

func NewCommandIO(writeKey, writeAuthKey, readKey, readAuthKey []byte, conn io.ReadWriter) *commandIO {
	ret := new(commandIO)
	ret.writeAuth = hmac.New(sha256.New, writeAuthKey)
	ret.readAuth = hmac.New(sha256.New, readAuthKey)
	ret.conn = conn
	ret.writeLock = new(sync.Mutex)

	writeBlkCipher, _ := aes.NewCipher(writeKey)
	readBlkCipher, _ := aes.NewCipher(readKey)

	// IV: 0 for all. Since we change keys for each connection, letting IV=0 won't hurt.
	writeIV := make([]byte, writeBlkCipher.BlockSize())
	readIV := make([]byte, readBlkCipher.BlockSize())

	writeStream := cipher.NewCTR(writeBlkCipher, writeIV)
	readStream := cipher.NewCTR(readBlkCipher, readIV)

	// Then for each encrypted bit,
	// it will be written to both the connection and the hmac
	// We use encrypt-then-hmac scheme.
	mwriter := io.MultiWriter(conn, ret.writeAuth)
	swriter := new(cipher.StreamWriter)
	swriter.S = writeStream
	swriter.W = mwriter
	ret.cryptWriter = swriter

	// Similarly, for each bit read from the connection,
	// it will be written to the hmac as well.
	tee := io.TeeReader(conn, ret.readAuth)
	sreader := new(cipher.StreamReader)
	sreader.S = readStream
	sreader.R = tee
	ret.cryptReader = sreader
	return ret
}
