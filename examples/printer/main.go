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
	"github.com/gorilla/mux"
	"net/http"
	"fmt"
	"bufio"
)

func PrintData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	fmt.Printf("%v:\n", r.URL.Path)

	reader := bufio.NewReader(r.Body)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			return
		}
		fmt.Printf("\t%v\n", string(line))
	}
	w.WriteHeader(200)
	return
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/auth", PrintData)
	r.HandleFunc("/msg", PrintData)
	r.HandleFunc("/fwd", PrintData)
	r.HandleFunc("/err", PrintData)
	r.HandleFunc("/login", PrintData)
	r.HandleFunc("/logout", PrintData)
	r.HandleFunc("/subscribe", PrintData)
	r.HandleFunc("/unsubscribe", PrintData)
	r.HandleFunc("/push", PrintData)
	http.Handle("/", r)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	return
}



