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

package kickit


import (
	"fmt"
	"strings"
)


// Available KickitCommand Actions.
const (
	CMD_UP = iota
	CMD_DOWN
	CMD_RESTART
)


// Available KickitResponse Statuses.
const (
	RES_SUCCESS = iota
	RES_ERROR
)


// KickitCommand objects can be passed to control Kickit services.
type KickitCommand struct {
	Action		int
	Service		string
	Args		map[string] string
	Response	chan *KickitResponse
}


// KickitResponse objects are passed to convey the result of a KickitCommand.
type KickitResponse struct {
	Status		int
	Message		string
	Data		map[string] string
}


// Construct a new KickitCommand.
func NewCommand(action int, service string, needResponse bool) (cmd *KickitCommand) {
	cmd = new(KickitCommand)
	cmd.Action = action
	cmd.Service = service
	cmd.Args = make(map[string] string)

	if needResponse {
		cmd.Response = make(chan *KickitResponse)
	}

	return cmd
}


// Construct a new KickitResponse.
func NewResponse(status int, message string) (res *KickitResponse) {
	res = new(KickitResponse)
	res.Status = status
	res.Message = message
	res.Data = make(map[string] string)

	return res
}

// StringToActionName converts an action name (capitalization optional) into
// an action number. Returns -1 if action name is unrecognized.
func StringToAction(actionName string) (action int) {
	capitalized := strings.ToUpper(actionName)

	switch capitalized {
	case "START":
		return CMD_UP
	case "STOP":
		return CMD_DOWN
	case "RESTART":
		return CMD_RESTART
	}

	return -1
}


// StatusToString converts a status number into a string. Panics on
// unrecognized value.
func StatusToString(status int) (statusString string) {
	switch status {
	case RES_ERROR:
		return "ERROR"
	case RES_SUCCESS:
		return "SUCCESS"
	}

	panic(fmt.Sprintf("Invalid Status: %d", status))
}


