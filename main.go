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


package main

import (
	"github.com/uniqush/uniqush-conn/proto"
	"fmt"
	"net"
	"encoding/pem"
	"crypto/x509"
	"crypto/rsa"
	"io/ioutil"
)

func readPrivateKey(keyFileName string) (priv *rsa.PrivateKey, err error) {
	keyData, err := ioutil.ReadFile(keyFileName)
	if err != nil {
		return
	}

	b, _ := pem.Decode(keyData)
	priv, err := x509.ParsePKCS1PrivateKey(b.Bytes)
	if err != nil {
		return
	}
	return
}

func main() {
	ln, err := net.Listen("tcp", ":8964")
	if err != nil {
		fmt.Printf("Error: %v", err)
		return
	}
}

