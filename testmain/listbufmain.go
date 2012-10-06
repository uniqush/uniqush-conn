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
	"io"
	"fmt"
	"time"
	"sync"
)

func dataProducer(buf *ListBuffer, data []byte, stepSize int, report chan<- error) {
	for stepSize <= len(data) {
		s := stepSize
		if s > len(data) {
			s = len(data)
		}
		_, err := buf.Write(data[:s])
		if err != nil && err != ErrFull {
			report <- err
		}
		for err == ErrFull {
			fmt.Println("Producer is blocked because of limitation of space. Current data size = ", buf.Size())
			buf.WaitForSpace(s)
			fmt.Println("Producer got extra space")
			_, err = buf.Write(data[:s])
			if err != nil && err != ErrFull {
				report <- err
			}
		}
		data = data[stepSize:]
	}
	close(report)
}

func dataConsumer(buf *ListBuffer, data []byte, stepSize int, report chan<- error) {
	d := make([]byte, len(data))
	backup := data
	i := d
	for stepSize <= len(data) {
		s := stepSize
		if s > len(data) {
			s = len(data)
		}
		n, err := buf.Read(i[:s])
		if err != nil && err != ErrFull {
			report <- err
		}
		for err == io.EOF {
			fmt.Println("Consumer is blocked because there is no data, Current Data size = ", buf.Size())
			buf.WaitForData()
			fmt.Println("Consumer got extra data")
			n, err = buf.Read(i[:s])
			if err != nil && err != io.EOF {
				report <- err
			}
		}
		data = data[n:]
		i = i[n:]
	}
	for j := 0; j < len(backup); j++ {
		if backup[j] != d[j] {
			report <- fmt.Errorf("@ %v: data[i] = %v; received[i] = %v", j, backup[j], d[j])
		}
	}
	close(report)
}


func main() {
	fmt.Println("-------------------------")
	fmt.Println("-------------------------")
	lb := NewListBuffer(10)
	dl := 100
	data := make([]byte, dl)

	for i := 0; i < dl; i++ {
		data[i] = byte(i)
	}

	prodErrChan := make(chan error)
	consErrChan := make(chan error)

	go dataProducer(lb, data, 5, prodErrChan)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		for e := range prodErrChan {
			fmt.Errorf("Producer Error: %v", e)
		}
		wg.Done()
	}()
	time.Sleep(1 * time.Second)
	go dataConsumer(lb, data, 10, consErrChan)

	wg.Add(1)
	go func() {
		for e := range consErrChan {
			fmt.Errorf("Producer Error: %v", e)
		}
		wg.Done()
	}()
	wg.Wait()
}

