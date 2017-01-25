package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lxc/lxd"
	log "github.com/sirupsen/logrus"
)

const (
	// ERROR is logrus' error level
	ERROR = log.ErrorLevel
	// FATAL is logrus' fatal level
	FATAL = log.FatalLevel
	// WARN is logrus' warning level
	WARN = log.WarnLevel
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

// Copy copies files into/out of containers
func Copy(src, dst string) (int64, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	srcFileStat, err := srcFile.Stat()
	if err != nil {
		return 0, err
	}

	if !srcFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()
	return io.Copy(dstFile, srcFile)
}

func hasExtension(client *lxd.Client, extension string) bool {
	r := false
	s, _ := client.ServerStatus()
	for _, ext := range s.APIExtensions {
		if ext == extension {
			r = true
			break
		}
	}
	return r
}

func splitFilePath(path string) (contextPath, containerPath string, err error) {
	split := strings.SplitN(path, ":", 2)
	if len(split) != 2 {
		err = fmt.Errorf("Incorrect file path format: %s", path)
		return
	}
	contextPath = split[0]
	containerPath = split[1]
	return
}
