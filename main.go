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


package main

import (
	"github.com/uniqush/uniqush-conn/configparser"
	"github.com/uniqush/uniqush-conn/msgcenter"
	"fmt"
	"net"
	"encoding/pem"
	"crypto/x509"
	"crypto/rsa"
	"io/ioutil"
	"flag"
	"os"
	"time"
)

func readPrivateKey(keyFileName string) (priv *rsa.PrivateKey, err error) {
	keyData, err := ioutil.ReadFile(keyFileName)
	if err != nil {
		return
	}

	b, _ := pem.Decode(keyData)
	priv, err = x509.ParsePKCS1PrivateKey(b.Bytes)
	if err != nil {
		return
	}
	return
}

var argvKeyFile = flag.String("key", "key.pem", "private key")
var argvConfigFile = flag.String("config", "config.yaml", "config file path")

// In memory of the blood on the square.
var argvPort = flag.Int("port", 0x2304, "port number")

func main() {
	flag.Parse()
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", *argvPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Network error: %v\n", err)
		return
	}

	privkey, err := readPrivateKey(*argvKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Key error: %v\n", err)
		return
	}
	config, err := configparser.Parse(*argvConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		return
	}

	center := msgcenter.NewMessageCenter(ln, privkey, nil, nil, 3 * time.Second, config.Auth, config)
	center.Start()
}

