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
	"log"
	"path"
)


const (
	CTRLFILE	= ".kcontrol"
)


// Kickit is a context for the Kickit process management system.
type Kickit struct {
	Root			string
	CtrlPath		string
	CtrlChan		chan *KickitCommand
	DeadChan		chan string
	children		map[string] *KickitProcess
}


// NewKickit constructs a new Kickit context. The specified root is the
// services directory in which this Kickit context should operate.
func NewKickit(root string) (ctxt *Kickit) {
	return &Kickit{
		Root:		root,
		CtrlPath:	path.Join(root, CTRLFILE),
		CtrlChan:	make(chan *KickitCommand),
		DeadChan:	make(chan string),
		children:	make(map[string] *KickitProcess),
	}
}


// Run begins execution in this Kickit context.
func (ctxt *Kickit) Run() {
	err := ListenControl(ctxt.CtrlChan, "unix", ctxt.CtrlPath)
	if err != nil {
		log.Printf("error: %s\n", err)
		return
	}

	log.Printf("listening: %s\n", ctxt.CtrlPath)

	for {
		select {
		case cmd := <-ctxt.CtrlChan:
			ctxt.handleCommand(cmd)
		case name := <-ctxt.DeadChan:
			ctxt.removeChild(name)
		}
	}
}


// handleCommand handles incomming KickitCommands. This is primarily a matter
// of routing the command to the specified service, but it can also require
// instantiation of new KickitProcess objects.
func (ctxt *Kickit) handleCommand(cmd *KickitCommand) {
	child, present := ctxt.children[cmd.Service]

	// 'START' or 'RESTART' commands can create new child processes
	if !present && (cmd.Action == CMD_UP || cmd.Action == CMD_RESTART) {
		child = NewKickitProcess(ctxt, cmd.Service)
		ctxt.children[cmd.Service] = child
		log.Printf("%s created\n", cmd.Service)
	} else if !present {
		res := NewResponse(RES_ERROR, "Service does not exist")
		cmd.Response <- res
		return
	}

	child.CmdChan <- cmd
}


// removeChild removes a KickitProcess from the children map. This should
// be called from Run in response to a message from a child process.
func (ctxt *Kickit) removeChild(name string) {
	ctxt.children[name] = nil, false
	log.Printf("%s removed\n", name)
}
