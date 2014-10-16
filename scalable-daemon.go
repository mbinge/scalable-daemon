package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func realPath(path string) string {
	if path[0] == '~' {
		home := os.Getenv("HOME")
		path = home + path[1:]
	}

	rpath, err := filepath.Abs(path)
	if err == nil {
		path = rpath
	}
	return strings.TrimSpace(path)
}

func getLogAndPid() (string, string) {
	cfg := LoadCfg()
	bindir := filepath.Dir(os.Args[0])
	piddir := realPath(filepath.Dir(bindir + "/" + cfg.Pid))
	err := os.MkdirAll(piddir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	}
	logdir := realPath(filepath.Dir(bindir + "/" + cfg.Log))
	err1 := os.MkdirAll(logdir, os.ModePerm)
	if err1 != nil {
		log.Fatal(err1)
		os.Exit(-1)
	}
	return bindir + "/" + cfg.Pid, bindir + "/" + cfg.Log
}

type List []string

func (left List) sub(right List) List {
	llen := len(left)
	rlen := len(right)
	left_only := make(List, 0)
	for i := 0; i < llen; i++ {
		litem := left[i]
		hit := false
		for j := 0; j < rlen; j++ {
			ritem := right[j]
			if litem == ritem {
				hit = true
			}
		}
		if hit == false {
			left_only = append(left_only, litem)
		}
	}
	return left_only
}

var waiting bool

func waitExit(list map[int]*Proc, cmd string) {
	for {
		waiting = true
		if len(list) == 0 {
			waiting = false
			break
		}
		for i, _ := range list {
			if list[i].wait() == true {
				log.Println("Stopped:", cmd, i)
				delete(list, i)
			}
		}
		time.Sleep(time.Millisecond * 500)
	}
}

func _init(stop chan struct{}) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	working_cfg := refreshCfg()
	monitor := new(Monitor)
	monitor.Init()
	go monitor.Start()

	waiting = false
	//kill old task before run
	snap_cfg := working_cfg.ReadSnap()
	for index, _ := range snap_cfg.Tasks {
		for _, oproc := range snap_cfg.Tasks[index].Instances {
			if proc, err := os.FindProcess(oproc.Pid); err == nil {
				proc.Kill()
				proc.Wait()
			}
		}
	}

	//run task in config
	for index, _ := range working_cfg.Tasks {
		task := working_cfg.Tasks[index]
		for i := 0; i < task.Parallel; i++ {
			log.Println("Start Cmd:", task.Cmd, i)
			task.exec(i)
		}
		for _, dir := range task.AutoAffect {
			monitor.AddWatch(dir, []int{index})
		}
	}

	ticker := time.NewTicker(time.Second * time.Duration(10))
	tickerLive := time.NewTicker(time.Second * time.Duration(1))
	last_notify := make(map[int]time.Time)
	last_tick_one := time.Now()
	zero_time := time.Time{}
EXIT:
	for {
		select {
		case <-stop:
			log.Println("receive stop")
			for index, _ := range working_cfg.Tasks {
				task := working_cfg.Tasks[index]
				for i := 0; i < task.Parallel; i++ {
					task.kill(i)
				}
				for _, dir := range task.AutoAffect {
					monitor.DelWatch(dir, []int{index})
				}
			}
			break EXIT
		case <-ticker.C:
			if waiting == true {
				log.Println("waiting job finished")
				continue
			}
			//Refresh Configure, Synchronously modify tasks automatically
			log.Println("ticker refresh Config")
			cfg := refreshCfg()
			wIndex := make(map[string]int)
			for index, _ := range working_cfg.Tasks {
				wIndex[working_cfg.Tasks[index].Cmd] = index
			}
			for _, task := range cfg.Tasks {
				hit := false
				for index, _ := range working_cfg.Tasks {
					wtask := working_cfg.Tasks[index]
					if wtask.Cmd == task.Cmd {
						delete(wIndex, wtask.Cmd)
						wtask.KillGracefull = task.KillGracefull
						wtask.Stdout = task.Stdout
						//Add Instace
						waitList := make(map[int]*Proc)
						if wtask.Parallel < task.Parallel {
							for i := wtask.Parallel; i < task.Parallel; i++ {
								log.Println("Start Cmd:", wtask.Cmd, i)
								wtask.exec(i)
							}
						} else if wtask.Parallel > task.Parallel {
							for i := task.Parallel; i < wtask.Parallel; i++ {
								if wtask.KillGracefull == true {
									wtask.start_shrink(i)
									waitList[i] = wtask.Instances[i]
								} else {
									wtask.kill(i)
								}
								delete(wtask.Instances, i)
								log.Println("Start_Stop Cmd:", wtask.Cmd, i)
							}
							waitExit(waitList, wtask.Cmd)
						}
						wtask.Parallel = task.Parallel
						hit = true

						toDel := List(wtask.AutoAffect).sub(task.AutoAffect)
						toAdd := List(task.AutoAffect).sub(wtask.AutoAffect)
						for _, dir := range toAdd {
							monitor.AddWatch(dir, []int{index})
						}
						for index, dir := range toDel {
							monitor.DelWatch(dir, []int{index})
						}
						wtask.AutoAffect = task.AutoAffect
						break
					}
				}
				//New task
				if hit == false {
					working_cfg.Tasks = append(working_cfg.Tasks, task)
					index := len(working_cfg.Tasks)
					ntask := working_cfg.Tasks[index-1]
					for i := 0; i < ntask.Parallel; i++ {
						log.Println("Start Cmd:", ntask.Cmd, i)
						ntask.exec(i)
					}
					for _, dir := range ntask.AutoAffect {
						monitor.AddWatch(dir, []int{index})
					}
				}
			}
			waitList := make(map[int]*Proc)
			for _, index := range wIndex {
				task := working_cfg.Tasks[index]
				for i := 0; i < task.Parallel; i++ {
					if task.KillGracefull == true {
						task.start_stop(i)
						waitList[i] = task.Instances[i]
					} else {
						task.kill(i)
					}
					log.Println("Start_Stop Cmd:", task.Cmd, i)
				}
				waitExit(waitList, task.Cmd)
				for _, dir := range task.AutoAffect {
					monitor.DelWatch(dir, []int{index})
				}
				working_cfg.Tasks = append(working_cfg.Tasks[:index], working_cfg.Tasks[index+1:]...)
			}
			working_cfg.WriteSnap()
		case <-tickerLive.C:
			if waiting == true {
				log.Println("waiting job finished")
				continue
			}
			if time.Now().Sub(last_tick_one).Nanoseconds() < int64(time.Millisecond)*990 {
				continue
			}
			len_task := len(working_cfg.Tasks)
			hit_restart := false
			//restart task if received file change notification
			for i := 0; i < len_task; i++ {
				tasks := working_cfg.Tasks[i]
				if last_notify[i] != zero_time && time.Now().Sub(last_notify[i]).Seconds() > 5.0 {
					ins := tasks.Instances
					for index, proc := range ins {
						if proc.process != nil {
							log.Println("Restart Cmd:", tasks.Cmd, index)
							working_cfg.Tasks[i].restart(index)
							hit_restart = true
						}
					}
					last_notify[i] = zero_time
				}
			}
			if hit_restart == true {
				log.Println("skip check proc life:", len(tickerLive.C))
				last_tick_one = time.Now()
				continue
			}
			//check subproc is still alive
			for i := 0; i < len_task; i++ {
				tasks := working_cfg.Tasks[i]
				ins := tasks.Instances
				for index, proc := range ins {
					wpid, _ := syscall.Wait4(proc.Pid, nil, syscall.WNOHANG, nil)
					if wpid != 0 {
						log.Println("Restart Cmd:", tasks.Cmd, index)
						tasks.exec(index)
					}
				}
			}
			last_tick_one = time.Now()
		case index := <-monitor.RestartC:
			//record notification event
			last_notify[index] = time.Now()
		}
	}
}

//func main() {
//	stoper := make(chan struct{})
//	_init(stoper)
//}
