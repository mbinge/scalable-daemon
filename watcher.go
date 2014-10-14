package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/fsnotify.v1"
)

type Monitor struct {
	Dirs     map[string]map[int]bool
	watcher  *fsnotify.Watcher
	done     chan bool
	RestartC chan int
}

func (monitor *Monitor) delWatchRecursively(dir string) error {
	hit := false
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if f != nil && f.IsDir() {
			log.Println("DEL:", path)
			err := monitor.watcher.Remove(path)
			hit = true
			if err != nil {
				log.Println(err)
			}
		}
		return nil
	})
	if hit == false {
		log.Println("DEL:", dir)
		err := monitor.watcher.Remove(dir)
		if err != nil {
			log.Println(err)
		}
	}
	return err
}

func (monitor *Monitor) DelWatch(dir string, item []int) {
	if _, ok := monitor.Dirs[dir]; ok == true {
		for in := range item {
			delete(monitor.Dirs[dir], in)
		}
		if len(monitor.Dirs[dir]) == 0 {
			monitor.delWatchRecursively(dir)
		}
	}
}

func (monitor *Monitor) addWatchRecursively(dir string) error {
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if f != nil && f.IsDir() {
			log.Println("ADD:", path)
			err := monitor.watcher.Add(path)
			if err != nil {
				log.Println(err)
			}
		}
		return nil
	})
	return err
}

func (monitor *Monitor) AddWatch(dir string, item []int) {
	if _, ok := monitor.Dirs[dir]; ok != true {
		monitor.Dirs[dir] = make(map[int]bool)
		monitor.addWatchRecursively(dir)
	}
	for in := range item {
		monitor.Dirs[dir][in] = true
	}
}

func (monitor *Monitor) Init() {
	var err error
	monitor.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	monitor.Dirs = make(map[string]map[int]bool)
	monitor.done = make(chan bool)
	monitor.RestartC = make(chan int)
}

func (monitor *Monitor) Start() {
	for {
		select {
		case event := <-monitor.watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				file, err := os.Stat(event.Name)
				if err != nil {
					log.Println(err)
				} else if file.IsDir() {
					monitor.addWatchRecursively(event.Name)
				}
			}
			for dir, items := range monitor.Dirs {
				if strings.HasPrefix(event.Name, dir) {
					for item, _ := range items {
						log.Println("SEND-RESTART:", item)
						monitor.RestartC <- item
					}
				}
			}
		case err := <-monitor.watcher.Errors:
			log.Println(err)
		case <-monitor.done:
			for dir, _ := range monitor.Dirs {
				monitor.delWatchRecursively(dir)
				delete(monitor.Dirs, dir)
			}
			monitor.watcher.Close()
			monitor.done <- true
			break
		}
	}
}

func (monitor *Monitor) Stop() {
	monitor.done <- true
	<-monitor.done
}

func main_test() {
	monitor := new(Monitor)
	monitor.Init()
	go monitor.Start()
	monitor.AddWatch("/Users/champion/work/go/src/scalable-daemon/a", []int{1, 2})
	monitor.AddWatch("/Users/champion/work/go/src/scalable-daemon/b", []int{1, 3})
	now := time.Now()
EXIT:
	for {
		select {
		case item := <-monitor.RestartC:
			log.Println("Should Restart:", item)
		default:
			cur := time.Now()
			if cur.Sub(now).Seconds() > 1000 {
				monitor.Stop()
				break EXIT
			}
		}
	}
}
