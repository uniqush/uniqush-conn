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
	"fmt"
	"net"
	"crypto/sha256"
	"crypto/rsa"
	"crypto/rand"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"io"
	"errors"
)

var ErrZeroEntropy = errors.New("Need more random number")
var ErrBadServer = errors.New("Unkown Server")
var ErrCorruptedData = errors.New("Corrupted Data")

func clientAuthenticate(conn net.Conn, pubKey *rsa.PublicKey, username, token string) error {
	// This makes 1024 bit pub key impossible.
	dataLen := 128
	data := make([]byte, dataLen)
	n, err := io.ReadFull(rand.Reader, data)
	if err != nil {
		return err
	}
	if n != len(data) && n > sessionKeyLen + macKeyLen {
		data = data[:n]
	}
	if n != len(data) {
		return ErrZeroEntropy
	}
	sha := sha256.New()

	fmt.Printf("Message sent: %v\n", data)
	out, err := rsa.EncryptOAEP(sha, rand.Reader, pubKey, data, nil)
	if err != nil {
		return err
	}
	writen(conn, out)

	// Wait the server.
	// It should be able to decrypt the message and send back the
	// random salt.
	saltLen := len(data) - sessionKeyLen - macKeyLen - ivLen
	cipherText := make([]byte, saltLen + hmacLen)
	n, err = io.ReadFull(conn, cipherText)
	if err != nil {
		return err
	}
	if n != len(cipherText) {
		return ErrBadServer
	}
	sessionKey := data[:sessionKeyLen]
	macKey := data[sessionKeyLen:sessionKeyLen + macKeyLen]
	iv := data[sessionKeyLen + macKeyLen:sessionKeyLen + macKeyLen + ivLen]
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return err
	}
	fmt.Printf("block size: %v\n", block.BlockSize())
	stream := cipher.NewCTR(block, iv)
	mac := hmac.New(sha256.New, macKey)
	err = writen(mac, cipherText[hmacLen:])
	if err != nil {
		return err
	}
	hmacSum := mac.Sum(nil)
	mac.Reset()
	for i, b := range hmacSum {
		if b != cipherText[i] {
			return ErrCorruptedData
		}
	}

	salt := cipherText[hmacLen:]
	stream.XORKeyStream(salt, salt)
	fmt.Printf("salt: len = %v; %v\n", len(salt), salt)

	for i, b := range data[sessionKeyLen + macKeyLen + ivLen:] {
		if b != salt[i] {
			return ErrBadServer
		}
	}
	return nil
}

func Dial(conn net.Conn, pubKey *rsa.PublicKey, username, token string) (ret net.Conn, err error) {
	err = clientAuthenticate(conn, pubKey, username, token)
	if err != nil {
		return
	}
	ret = conn
	return
}

