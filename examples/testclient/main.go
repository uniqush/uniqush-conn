package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/uniqush/uniqush-conn/proto"
	"github.com/uniqush/uniqush-conn/proto/client"
	"io/ioutil"
	"io"
	"net"
	"os"
	"time"
	"bufio"
	"strings"
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

func messagePrinter(msgChan <-chan *proto.Message, digestChan <-chan *client.Digest) {
	for {
		select {
		case msg := <-msgChan:
			if msg == nil {
				return
			}
			fmt.Printf("- [Service=%v][Sender=%v][Id=%v]", msg.SenderService, msg.Sender, msg.Id)
			for k, v := range msg.Header {
				fmt.Printf("[%v=%v]", k, v)
			}
			if msg.Body != nil {
				fmt.Printf("%v", string(msg.Body))
			}
			fmt.Printf("\n")
		case digest := <-digestChan:
			if digest == nil {
				return
			}
			fmt.Printf("- Digest:%v\n", digest)
		}
	}
}

func messageReceiver(conn client.Conn, msgChan chan<- *proto.Message) {
	defer conn.Close()
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
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
		msg := new(proto.Message)

		elems := strings.SplitN(line, ":", 2)
		if len(elems) == 2 {
			msg.Body = []byte(elems[1])
			err = conn.ForwardRequest(elems[0], conn.Service(), msg)
		} else {
			msg.Body = []byte(line)
			err = conn.SendMessage(msg)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
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
	addr := "127.0.0.1:8989"
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
		fmt.Fprintf(os.Stderr, "Invalid address: %v\n", err)
		return
	}
	conn, err := client.Dial(c, pk, *argvService, *argvUsername, *argvPassword, 3*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid address: %v\n", err)
		return
	}

	msgChan := make(chan *proto.Message)
	digestChan := make(chan *client.Digest)
	conn.SetDigestChannel(digestChan)
	go messageReceiver(conn, msgChan)
	go messagePrinter(msgChan, digestChan)
	messageSender(conn)
}
