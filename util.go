package main

import (
	log "github.com/sirupsen/logrus"
	"os"
)

const (
	ERROR = log.ErrorLevel
	FATAL = log.FatalLevel
	WARN  = log.WarnLevel
)

func asrt(iface interface{}, err error) interface{} {
	if err != nil {
		log.Errorf("ERROR: %v", err)
	}
	return iface
}

func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func dirExists(name string) bool {
	if d, err := os.Stat(name); err != nil || !d.IsDir() {
		return false
	}
	return true
}
