package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type Proc struct {
	process *os.Process
	stdin   *io.WriteCloser
	Pid     int
}

type Task struct {
	Cmd           string   `json:"cmd"`
	Home          string   `json:"home"`
	AutoAffect    []string `json:"autoAffect"`
	KillGracefull bool     `json:"killGracefull"`
	Parallel      int
	Stdout        string
	Instances     map[int]*Proc
}

func (tasks *Task) start_stop(index int) {
	if task, ok := tasks.Instances[index]; ok == true {
		in := *task.stdin
		in.Write([]byte("STOP\n"))
	}
}

func (tasks *Task) restart(index int) {
	if _, ok := tasks.Instances[index]; ok == true {
		tasks.kill(index)
		tasks.exec(index)
	}
}

func (tasks *Task) start_shrink(index int) {
	if task, ok := tasks.Instances[index]; ok == true {
		in := *task.stdin
		in.Write([]byte("SHRINK\n"))
	}
}

func (proc *Proc) wait() bool {
	wpid, _ := syscall.Wait4(proc.Pid, nil, syscall.WNOHANG, nil)
	if wpid != 0 {
		return true
	}
	return false
}

func (tasks *Task) kill(index int) {
	if task, ok := tasks.Instances[index]; ok == true {
		task.process.Kill()
		task.process.Wait()
	}
}

func (tasks *Task) exec(index int) bool {
	cmd := tasks.Cmd + " " + strconv.Itoa(index)
	cmds := strings.Split(cmd, " ")
	hdl := exec.Command(cmds[0], cmds[1:]...)
	hdl.Dir = tasks.Home
	if len(tasks.Stdout) > 0 {
		if out, err := os.OpenFile(tasks.Stdout, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm); err == nil {
			hdl.Stdout = out
			hdl.Stderr = out
		}
	}
	stdin, err := hdl.StdinPipe()
	if err != nil {
		log.Println(err)
	}
	hdl.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	err4 := hdl.Start()
	if err4 != nil {
		log.Println(err4)
		return false
	}
	if tasks.Instances == nil {
		tasks.Instances = make(map[int]*Proc)
	}
	proc := new(Proc)
	proc.process = hdl.Process
	proc.Pid = hdl.Process.Pid
	proc.stdin = &stdin
	tasks.Instances[index] = proc
	return true
}
