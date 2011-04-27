/*
 * Licensed to Rackspace, Inc ('Rackspace') under one or more contributor
 * license agreements.  See the NOTICE file distributed with this work for
 * additional information regarding copyright ownership.  Rackspace licenses
 * this file to You under the Apache License, Version 2.0 (the "License"); you
 * may not use this file except in compliance with the License.  You may obtain
 * a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
 * License for the specific language governing permissions and limitations
 * under the License.
 */

package main


import (
	"os"
	"fmt"
	"path/filepath"
	"kickit"
)


var (
	ErrBadRoot	= os.NewError("error: specified service-dir is not accessible")
)


func getRoot(passed string) (root string, err os.Error) {
	if filepath.IsAbs(passed) {
		root = passed
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = filepath.Join(cwd, passed)
	}

	root = filepath.Clean(root)
	rootstat, err := os.Stat(root)

	if err != nil || !rootstat.IsDirectory() {
		return "", ErrBadRoot
	}

	return root, nil
}


func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: kickit service-dir\n")
		os.Exit(1)
	}

	root, err := getRoot(os.Args[1])

	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}

	kickitContext := kickit.NewKickit(root)
	kickitContext.Run()
}
