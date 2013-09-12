package main

import (
	"bufio"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/uniqush/uniqush-conn/rpc"

	"github.com/uniqush/uniqush-conn/proto/client"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"
)

func loadRSAPublicKey(keyFileName string) (rsapub *rsa.PublicKey, err error) {
	keyData, err := ioutil.ReadFile(keyFileName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	b, _ := pem.Decode(keyData)
	if b == nil {
		err = fmt.Errorf("No key in the file")
		return
	}
	key, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	rsapub, ok := key.(*rsa.PublicKey)

	if !ok {
		err = fmt.Errorf("Not an RSA public key")
		return
	}
	return
}

var argvPubKey = flag.String("key", "pub.pem", "public key file")
var argvService = flag.String("s", "service", "service")
var argvUsername = flag.String("u", "username", "username")
var argvPassword = flag.String("p", "", "password")
var argvDigestThrd = flag.Int("d", 512, "digest threshold")
var argvCompressThrd = flag.Int("c", 1024, "compress threshold")

func messagePrinter(conn client.Conn, msgChan <-chan *rpc.MessageContainer, digestChan <-chan *client.Digest) {
	encoder := json.NewEncoder(os.Stdout)
	for {
		select {
		case msg := <-msgChan:
			if msg == nil {
				return
			}
			encoder.Encode(msg)
			if msg.Message.Body != nil {
				fmt.Printf("\n%v: %v", msg.Sender, string(msg.Message.Body))
			} else {
				fmt.Printf("\n")
			}
		case digest := <-digestChan:
			if digest == nil {
				return
			}
			encoder.Encode(digest)
			fmt.Printf("\n")
			conn.RequestMessage(digest.MsgId)
		}
	}
}

func messageReceiver(conn client.Conn, msgChan chan<- *rpc.MessageContainer) {
	defer conn.Close()
	for {
		msg, err := conn.ReceiveMessage()
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			return
		}
		msgChan <- msg
	}
}

func messageSender(conn client.Conn) {
	stdin := bufio.NewReader(os.Stdin)
	for {
		line, err := stdin.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			return
		}
		msg := &rpc.Message{}

		elems := strings.SplitN(line, ":", 2)
		if len(elems) == 2 {
			msg.Body = []byte(elems[1])

			recvers := strings.Split(elems[0], ",")
			err = conn.SendMessageToUsers(msg, 1*time.Hour, conn.Service(), recvers...)
		} else {
			msg.Body = []byte(line)
			err = conn.SendMessageToServer(msg)
		}
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			return
		}
	}
}

func main() {
	flag.Parse()
	pk, err := loadRSAPublicKey(*argvPubKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	addr := "127.0.0.1:8964"
	if flag.NArg() > 0 {
		addr = flag.Arg(0)
		_, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid address: %v\n", err)
			return
		}
	}

	c, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	conn, err := client.Dial(c, pk, *argvService, *argvUsername, *argvPassword, 3*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login Error: %v\n", err)
		return
	}
	err = conn.Config(*argvDigestThrd, *argvCompressThrd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config Error: %v\n", err)
		return
	}

	msgChan := make(chan *rpc.MessageContainer)
	digestChan := make(chan *client.Digest)
	conn.SetDigestChannel(digestChan)
	go messageReceiver(conn, msgChan)
	go messagePrinter(conn, msgChan, digestChan)
	messageSender(conn)
}
