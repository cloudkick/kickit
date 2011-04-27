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
	"syscall"
	"path"
	"log"
	"fmt"
	"time"
)


const (
	RUNFILE			= "run"
	LOGFILE			= "current"
	DEATHPAUSE		= 5000000000
	KILLPAUSE		= 5000000000
	TOOYOUNGTODIE	= 5000000000
)


var (
	LOGDIR			= path.Join("log", "main")
)


// Process Errors
var (
	ErrNoProcRoot		= os.NewError("process: service root does not exist")
	ErrNotRunning		= os.NewError("process: process not running")
	ErrCreatingLogDir	= os.NewError("process: error creating log directory")
	ErrOpeningLogFile	= os.NewError("process: error creating log file")
)


// KickProcess objects are used to keep track of processes that kickit
// is currently executing.
type KickitProcess struct {
	name			string
	runDir			string
	runPath			string
	wantDown		bool
	process			*os.Process
	startedAt		int64
	waitChan		chan *os.Waitmsg
	deadChan		chan string
	CmdChan			chan *KickitCommand
}


// NewKickitProcess creates a new KickitProcess by starting the associated
// process, then returns the KickitProcess object. It also spawns a goroutine
// that monitor's the processes health and, if the process dies, restarts it
func NewKickitProcess(ctxt *Kickit, name string) (kp *KickitProcess) {
	runDir	:= path.Join(ctxt.Root, name);
	runPath := path.Join(runDir, RUNFILE)

	kp =  &KickitProcess{
		name:		name,
		runDir:		runDir,
		runPath:	runPath,
		wantDown:	false,
		process:	nil,
		waitChan:	make(chan *os.Waitmsg),
		deadChan:	ctxt.DeadChan,
		CmdChan:	make(chan *KickitCommand),
	}

	go kp.run()

	return kp
}


// run is the main loop for a KickitProcess. It waits a command to arrive or
// the process to die.
func (kp *KickitProcess) run() {
	for {
		// Cause this process to be removed from the Kickit context
		if kp.wantDown {
			kp.remove()
			break
		}

		select {
		case cmd := <-kp.CmdChan:
			kp.handleCommand(cmd)
		case <- kp.waitChan:
			kp.uponDeath()
		}
	}
}


// uponDeath handles unexpected process deaths.
func (kp *KickitProcess) uponDeath() {
	kp.process = nil

	// When a very young process dies, wait a few seconds before restarting
	lifespan := time.Nanoseconds() - kp.startedAt
	if lifespan < TOOYOUNGTODIE {
		time.Sleep(DEATHPAUSE)
	}

	kp.start()
}


// handleCommand handles received KickitCommands.
func (kp *KickitProcess) handleCommand(cmd *KickitCommand) {
	var res *KickitResponse;

	switch cmd.Action {
	case CMD_UP:
		pid, err := kp.start()
		if err != nil {
			res = NewResponse(RES_ERROR, err.String())
			break
		}
		res = NewResponse(RES_SUCCESS, "Service started")
		res.Data["pid"] = fmt.Sprintf("%d", pid)

	case CMD_DOWN:
		msg, err := kp.stop()
		if err != nil {
			res = NewResponse(RES_ERROR, err.String())
			break
		}
		res = NewResponse(RES_SUCCESS, "Service stopped")
		res.Data["pid"] = fmt.Sprintf("%d", msg.Pid)
		res.Data["msg"] = msg.String()

	case CMD_RESTART:
		// Attempt stop
		kp.stop()

		// Start
		pid, err := kp.start()
		if err != nil {
			res = NewResponse(RES_ERROR, err.String())
			break
		}
		res = NewResponse(RES_SUCCESS, "Service started")
		res.Data["pid"] = fmt.Sprintf("%d", pid)
	}

	cmd.Response <- res
}


// start starts the process, starts wait() in a goroutine, then returns the pid
// of the newly running process.
func (kp *KickitProcess) start() (pid int, err os.Error) {
	kp.wantDown = false

	// If the process is already running, so much the better
	if kp.process != nil {
		return kp.process.Pid, nil
	}

	// Make sure the process root actually exists
	if !pathIsDirectory(kp.runDir) {
		kp.remove()
		return -1, ErrNoProcRoot
	}

	// Build arguments and fds
	args := []string{
		kp.runPath,
	}
	fds, err := kp.getFds()

	if err != nil {
		kp.remove()
		return -1, err
	}

	// Start the process
	proc, err := os.StartProcess(kp.runPath, args, nil, kp.runDir, fds)
	if err != nil {
		log.Printf("%s start() error: %s\n", kp.name, err)
		kp.remove()
		return -1, err
	}

	kp.process = proc
	kp.startedAt = time.Nanoseconds()

	log.Printf("%s started\n", kp.name)
	go kp.wait()
	return proc.Pid, nil
}


// stop attempts to stop the process, first with SIGTERM, then if the process
// is still running after KILLPAUSE nanoseconds with SIGKILL. It returns the
// Waitmsg generated when the process died.
func (kp *KickitProcess) stop() (msg *os.Waitmsg, err os.Error) {
	kp.wantDown = true

	if kp.process == nil {
		return nil, ErrNotRunning
	}

	// If an error occurs here it indicates the process already died. This
	// is fine, as it means a Waitmsg is waiting for us when we hit the select
	syscall.Kill(kp.process.Pid, syscall.SIGTERM)

	// Give the process KILLPAUSE ns to die
	timer := time.NewTimer(KILLPAUSE)
	select {
	case msg = <-kp.waitChan:
		timer.Stop()
		log.Printf("%s stopped\n", kp.name)
		kp.process = nil
		return msg, nil

	case <-timer.C:
	}

	// Same situation, an error means the process already died - its ok
	syscall.Kill(kp.process.Pid, syscall.SIGKILL)
	msg = <-kp.waitChan
	log.Printf("%s stopped\n", kp.name)
	kp.process = nil
	return msg, nil
}


// wait waits for the process to exit then writes the Waitmsg to the
// process's waitChan.
func (kp *KickitProcess) wait() {
	msg, err := kp.process.Wait(0)

	// An error here would be *reall* bad - can it happen?
	if err != nil {
		kp.logError(err)
		return
	}

	log.Printf("%s exited: %s\n", kp.name, msg.String())
	kp.waitChan <- msg
}


// getFds generates an array of three pointers to os.File objects corresponding
// to stdin, stdout and stderr for this process.
func (kp *KickitProcess) getFds() (fds []*os.File, err os.Error) {
	logDirAbs := path.Join(kp.runDir, LOGDIR)
	logPathAbs := path.Join(logDirAbs, LOGFILE)

	// Create the log directory (if necessary)
	err = os.MkdirAll(logDirAbs, 0755)
	if err != nil {
		log.Printf("%s error: %s\n", kp.name, err)
		return nil, ErrCreatingLogDir
	}

	// Open the log file
	flags := os.O_CREAT | os.O_APPEND | os.O_WRONLY
	logFileFd, err := os.Open(logPathAbs, flags, 0755)

	if err != nil {
		kp.logError(err)
		return nil, ErrOpeningLogFile
	}

	// Open /dev/null
	devNull, err := os.Open(os.DevNull, os.O_RDWR, 0)

	if err != nil {
		kp.logError(err)
		return nil, err
	}

	return []*os.File{
		devNull,
		logFileFd,
		os.Stderr,
	}, nil
}


// remove removes this process from the parent context. Naturally, any
// goroutines running should be stopped.
func (kp *KickitProcess) remove() {
	kp.deadChan <- kp.name
}


// logError logs an error on this process.
func (kp *KickitProcess) logError(err os.Error) {
	log.Printf("%s error: %s\n", kp.name, err)
}
