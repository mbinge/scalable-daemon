package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
)

type Configure struct {
	Log   string  `json:"log"`
	Pid   string  `json:"pid"`
	Snap  string  `json:"snap"`
	Tasks []*Task `json:"tasks"`
}

var (
	configure = flag.String("c", "config.json", "Configuration file")
)

func (cfg *Configure) ReadFrom(file string) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		log.Fatal(err)
	}
}

func (cfg *Configure) WriteSnap() {
	var result bytes.Buffer
	enc := gob.NewEncoder(&result)
	err := enc.Encode(cfg)
	if err != nil {
		log.Println("encode error:", err)
	}
	if err := ioutil.WriteFile(cfg.Snap, result.Bytes(), os.ModePerm); err != nil {
		log.Println(err)
	}
}

func (cfg *Configure) ReadSnap() *Configure {
	ncfg := new(Configure)
	f, err := os.Open(cfg.Snap)
	if err != nil {
		log.Println(err)
	}
	dec := gob.NewDecoder(f)
	if err := dec.Decode(ncfg); err != nil {
		log.Println(err)
	}
	return ncfg
}

func refreshCfg() *Configure {
	cfg := new(Configure)
	cfg.ReadFrom(*configure)
	return cfg
}

func LoadCfg() *Configure {
	lstat, err := os.Lstat(*configure)
	if err != nil {
		log.Fatal(err)
	} else if lstat.Mode()&os.ModeType != 0 {
		log.Fatalf(`"%s" is not a text file`, lstat.Name())
		os.Exit(-1)
	}
	return refreshCfg()
}
