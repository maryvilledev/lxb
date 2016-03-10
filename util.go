package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
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

func Copy(src, dst string) (int64, error) {
	src_file, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer src_file.Close()

	src_file_stat, err := src_file.Stat()
	if err != nil {
		return 0, err
	}

	if !src_file_stat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	dst_file, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dst_file.Close()
	return io.Copy(dst_file, src_file)
}
