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
	"fmt"
)

type commandIO struct {
	writeAuth   hash.Hash
	cryptWriter io.Writer

	// we want to make
	// compress-then-encrypt
	// be the default configuration
	noWriteEncrypt  bool
	noWriteCompress bool

	readAuth       hash.Hash
	cryptReader    io.Reader
	noReadEncrypt  bool
	noReadCompress bool

	conn io.ReadWriter
}

func (self *commandIO) ReadCompressOn() {
	self.noReadCompress = false
}

func (self *commandIO) ReadEncryptOn() {
	self.noReadEncrypt = false
}

func (self *commandIO) ReadCompressOff() {
	self.noReadCompress = true
}

func (self *commandIO) ReadEncryptOff() {
	self.noReadEncrypt = true
}

func (self *commandIO) WriteCompressOn() {
	self.noWriteCompress = false
}
func (self *commandIO) WriteEncryptOn() {
	self.noWriteEncrypt = false
}

func (self *commandIO) WriteCompressOff() {
	self.noWriteCompress = true
}

func (self *commandIO) WriteEncryptOff() {
	self.noWriteEncrypt = true
}

func (self *commandIO) writeThenHmac(data []byte) (mac []byte, err error) {
	writer := self.cryptWriter
	if self.noWriteEncrypt {
		writer = self.conn
	} else {
		self.writeAuth.Reset()
	}
	err = writen(writer, data)
	if err != nil {
		return
	}
	if self.noWriteEncrypt {
		return
	}
	mac = self.writeAuth.Sum(nil)
	return
}

func (self *commandIO) readThenHmac(data []byte) (mac []byte, err error) {
	reader := self.cryptReader
	if self.noReadEncrypt {
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
	if self.noReadEncrypt {
		return
	}
	mac = self.readAuth.Sum(nil)
	return
}

func (self *commandIO) writeHmac(mac []byte) error {
	if self.noWriteEncrypt {
		return nil
	}
	return writen(self.conn, mac)
}

func (self *commandIO) readAndCmpHmac(mac []byte) error {
	if self.noReadEncrypt {
		return nil
	}
	macRecved := make([]byte, self.readAuth.BlockSize())
	n, err := io.ReadFull(self.conn, macRecved)
	fmt.Printf("Have read the HAMC: %v\n", macRecved)
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

func (self *commandIO) decodeCommand(data []byte) (cmd *command, err error) {
	decoded := data
	if !self.noReadCompress {
		decoded, err = snappy.Decode(nil, data)
		if err != nil {
			return
		}
	}
	cmd = new(command)
	err = bson.Unmarshal(decoded, cmd)
	if err != nil {
		return
	}
	return
}

func (self *commandIO) encodeCommand(cmd *command) (data []byte, err error) {
	bsonEncoded, err := bson.Marshal(cmd)
	if err != nil {
		return
	}

	data = bsonEncoded
	if !self.noWriteCompress {
		data, err = snappy.Encode(nil, bsonEncoded)
		if err != nil {
			return
		}
	}
	return
}

func (self *commandIO) WriteCommand(cmd *command) error {
	data, err := self.encodeCommand(cmd)
	fmt.Printf("Encoded data: %v\n", data)
	if err != nil {
		return err
	}
	var cmdLen uint16
	cmdLen = uint16(len(data))
	fmt.Printf("Need to write %v bytes of data\n", cmdLen)
	err = binary.Write(self.conn, binary.LittleEndian, cmdLen)
	mac, err := self.writeThenHmac(data)
	fmt.Printf("Writing HMAC: %v...\n", mac)
	if err != nil {
		return err
	}
	err = self.writeHmac(mac)
	if err != nil {
		return err
	}
	return nil
}

func (self *commandIO) ReadCommand() (cmd *command, err error) {
	var cmdLen uint16
	err = binary.Read(self.conn, binary.LittleEndian, &cmdLen)
	if err != nil {
		return
	}
	fmt.Printf("Need to read %v bytes of data\n", cmdLen)
	data := make([]byte, int(cmdLen))
	mac, err := self.readThenHmac(data)
	fmt.Printf("Have read data: %v\n", data)
	fmt.Printf("Calculated the HMAC of read data: %v\n", mac)
	if err != nil {
		return
	}
	err = self.readAndCmpHmac(mac)
	if err != nil {
		return
	}
	cmd, err = self.decodeCommand(data)
	return
}

func newCommandIO(writeKey, writeAuthKey, readKey, readAuthKey []byte, conn io.ReadWriter) *commandIO {
	ret := new(commandIO)
	ret.writeAuth = hmac.New(sha256.New, writeAuthKey)
	ret.readAuth = hmac.New(sha256.New, readAuthKey)
	ret.conn = conn

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
