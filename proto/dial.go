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
	"io"
	"errors"
)

var ErrZeroEntropy = errors.New("Need more random number")
var ErrBadServer = errors.New("Unkown Server")

func clientAuthenticate(conn net.Conn, pubKey *rsa.PublicKey, username, token string) error {
	dataLen := 96
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
	salt := make([]byte, len(data) - sessionKeyLen - macKeyLen)
	n, err = io.ReadFull(conn, salt)
	if err != nil {
		return err
	}
	if n != len(salt) {
		return ErrBadServer
	}
	fmt.Printf("salt: len = %v; %v\n", len(salt), salt)
	for i, b := range data[sessionKeyLen + macKeyLen:] {
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

