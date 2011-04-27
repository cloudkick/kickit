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
	"os"
	"net"
	"log"
	"bufio"
	"bytes"
)


const (
	CRLF = "\r\n"
)


// Parser States
const (
	COMMAND = iota
	KWARG
)


// Parser Errors
var (
	ErrInvalidCommandLine = os.NewError("control: invalid command line")
	ErrInvalidKeywordLine = os.NewError("control: invalid keyword line")
)


// KickitController provides a control interface to Kickit over a socket.
type KickitController struct {
	ctrl		chan *KickitCommand
	listener	net.Listener
}


// ListenControl starts a KickitController at the specified address or path,
// then parses KickitCommands from incomming connections.
func ListenControl(ctrl chan *KickitCommand, nett, laddr string) (err os.Error) {
	c := new(KickitController)
	c.ctrl = ctrl

	if nett == "unix" {
		os.Remove(laddr)
	}

	c.listener, err = net.Listen(nett, laddr)

	if err != nil {
		return err
	}

	go c.Accept()
	return nil
}


// Accept and handle connections on this KickitController's listener.
func (c *KickitController) Accept() {
	defer c.listener.Close()
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			log.Panicf("Accept: %s", err)
		}
		go c.handleConnection(conn)
	}
}


// readControlCommand reads a KickitCommand from a bufio.Reader.
func readControlCommand(b *bufio.Reader) (cmd *KickitCommand, err os.Error) {
	state := COMMAND

	for {
		line, err := readControlLine(b)
		if err != nil {
			return nil, err
		}

		if len(line) == 0 {
			if state == COMMAND {
				continue
			}
			break
		}

		switch state {
		case COMMAND:
			// Split the line around the first space
			sidx := bytes.Index(line, []byte {' '})
			if (sidx < 1) {
				return nil, ErrInvalidCommandLine
			}
			cmdAction := StringToAction(string(line[:sidx]))
			cmdService := string(line[sidx + 1:])
			cmd = NewCommand(cmdAction, cmdService, true)
			state = KWARG

		case KWARG:
			// Split the line around the first '='
			sidx := bytes.Index(line, []byte {'='})
			if (sidx < 1) {
				return nil, ErrInvalidKeywordLine
			}
			key := string(line[:sidx])
			val := string(line[sidx + 1:])
			cmd.Args[key] = val
			state = KWARG
		}
	}
	return cmd, nil
}


func readControlLine(b *bufio.Reader) (line []byte, err os.Error) {
	line, err = b.ReadBytes('\n')

	if err != nil {
		return nil, err
	}

	// Strip optional CR, required LF
	if len(line) > 1 && line[len(line) - 2] == '\r' {
		line = line[0:len(line) - 2]
	} else {
		line = line[0:len(line) - 1]
	}

	return line, nil
}

// Write a KickitResponse to a network connection.
func sendControlResponse(conn net.Conn, res *KickitResponse) (err os.Error) {
	buf := bytes.NewBufferString("")

	switch res.Status {
	case RES_ERROR:
		buf.WriteString("ERROR")

	case RES_SUCCESS:
		buf.WriteString("SUCCESS")
	}
	buf.WriteString(" ")
	buf.WriteString(res.Message)
	buf.WriteString(CRLF)

	for key, val := range res.Data {
		buf.WriteString(key)
		buf.WriteString("=")
		buf.WriteString(val)
		buf.WriteString(CRLF)
	}

	buf.WriteString(CRLF)

	// Write it
	_, err = conn.Write(buf.Bytes())
	return err
}


// Handle an accepted connection for this KickitController.
func (c *KickitController) handleConnection(conn net.Conn) {
	defer conn.Close()
	b := bufio.NewReader(conn)

	for {
		var res *KickitResponse

		cmd, err := readControlCommand(b)
		if err != nil {
			return
		}

		if cmd.Action < 0 {
			res = NewResponse(RES_ERROR, "Invalid Action")
		} else {
			c.ctrl <- cmd
			res = <-cmd.Response
		}

		err = sendControlResponse(conn, res)
		if err != nil {
			return
		}
	}
}
