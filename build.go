package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/lxc/lxd"
	"github.com/lxc/lxd/shared"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type Build struct {
	ID, Remote, imgId string
	spec              *BuildSpec
	client            *lxd.Client
}

func NewBuild(spec *BuildSpec, client *lxd.Client, remote string) *Build {
	return &Build{
		spec:   spec,
		client: client,
		ID:     spec.BaseImg + "-build-" + fmt.Sprintf("%v", time.Now().Unix()),
		Remote: remote,
	}
}

func (b *Build) Execute(keepContainer bool) error {
	farray := []func() error{
		b.createBuildContainer,
		b.startBuildContainer,
		b.copyFiles,
		b.runCommands,
		b.stopBuildContainer,
		b.saveImageFromContainer,
	}
	if !keepContainer {
		farray = append(farray, b.removeBuildContainer)
	}
	for _, step := range farray {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (b *Build) createBuildContainer() error {
	log.Infoln("Creating build container")
	resp, err := b.client.Init(
		b.ID,
		b.Remote,
		b.spec.BaseImg,
		&b.spec.BuildProfiles,
		b.spec.BuildConfig,
		false)
	if err != nil {
		log.Debugln("Failed during createBuildContainer")
		return err
	}

	return b.client.WaitForSuccess(resp.Operation)
}

func (b *Build) startBuildContainer() error {
	log.Debugln("Starting build container")
	var (
		err  error
		resp *lxd.Response
	)

	if resp, err = b.client.Action(b.ID, shared.Start, 30, false, false); err != nil {
		log.Debugln("Failed during startBuildContainer")
		return err
	}

	if err = b.client.WaitForSuccess(resp.Operation); err != nil {
		log.Debugln("Failed during startBuildContainer")
		return err
	}

	log.Infoln("Waiting for network connectivity")
	i := 0
	for {
		br := false
		time.Sleep(2500 * time.Millisecond)
		s, err := b.client.ContainerState(b.ID)
		if err != nil {
			log.Debugln("Error getting container status: %v", err)
			i++
			continue
		}
		// log.Debugf("[wait_for_net] status=%s IPs=%+v", s.Status, s.Ips)
		for name, netObj := range s.Network {
			// All we care about is that there is a non-loopback interface
			// that has a v4 IP and is up
			if name != "lo" && len(netObj.Addresses) > 0 && netObj.State == "up" {
				for _, a := range netObj.Addresses {
					if a.Family == "inet" && a.Address != "127.0.0.1" {
						br = true
						break
					}
				}
			}
		}
		i++
		if br || i > 20 {
			break
		}
	}
	return err
}

func (b *Build) stopBuildContainer() error {
	log.Infoln("Stopping build container")
	resp, err := b.client.Action(b.ID, shared.Stop, 30, true, false)
	if err != nil {
		log.Debugln("Failed during stopBuildContainer")
		return err
	}
	return b.client.WaitForSuccess(resp.Operation)
}

func (b *Build) saveImageFromContainer() error {
	log.Infoln("Creating image from build container")
	var err error
	b.imgId, err = b.client.ImageFromContainer(
		b.ID,
		b.spec.Public,
		b.spec.ImgAliases,
		b.spec.ImgProperties,
	)
	if err == nil {
		log.Infoln("Created image", b.imgId)
	}
	return err
}

func (b *Build) removeBuildContainer() error {
	log.Infoln("Removing build container")
	resp, err := b.client.Delete(b.ID)
	if err != nil {
		log.Debugln("Failed during removeBuildContainer")
		return err
	}
	return b.client.WaitForSuccess(resp.Operation)
}

func (b *Build) copyFiles() error {
	var err error
	log.Infoln("Pushing files into container")
	for _, fileString := range b.spec.Files {
		var (
			fInfo os.FileInfo
			f     *os.File
		)
		split := strings.SplitN(fileString, ":", 2)
		if len(split) != 2 {
			log.Warnf("Incorrect file path format: %s", f)
			continue
		}
		contextPath := split[0]
		containerPath := split[1]

		fInfo, err = os.Stat(contextPath)
		if err != nil {
			log.Warnf("There's a problem with %s: %v", contextPath, err)
			continue
		}

		if fInfo.IsDir() {
			files, err := ioutil.ReadDir(fInfo.Name())
			if err != nil {
				log.Warnf("Could not open %s: %v", contextPath, err)
				continue
			}
			for _, fi := range files {
				f, err = os.Open(contextPath + "/" + fi.Name())
				if err != nil {
					log.Warnf("Could not open %s: %v", contextPath, err)
					continue
				}
				if err = b.client.PushFile(b.ID, containerPath, 0, 0, fInfo.Mode(), f); err != nil {
					log.Errorf("Failed to push %s: %v", contextPath, err)
					continue
				}
				log.Debugf("Pushed %s to %s", fi.Name(), containerPath)
			}
		} else {
			f, err = os.Open(contextPath)
			if err != nil {
				log.Warnf("Could not open %s: %v", contextPath, err)
				continue
			}
			if err = b.client.PushFile(b.ID, containerPath, 0, 0, fInfo.Mode(), f); err != nil {
				log.Errorf("Failed to push %s: %v", contextPath, err)
				continue
			}
			log.Debugf("Pushed %s to %s", contextPath, containerPath)
		}
	}
	log.Debugln("Finished pushing files")
	return err
}

func (b *Build) runCommands() error {
	log.Infoln("Executing build commands")
	for _, c := range b.spec.Cmd {
		log.Debugf("Executing command: %s", c)
		if _, err := b.client.Exec(b.ID,
			[]string{"/bin/sh", "-c", c},
			b.spec.Env,
			nil,
			os.Stdout,
			os.Stderr,
			func(l *lxd.Client, w *websocket.Conn) {},
		); err != nil {
			log.Debugln("Failed during build command %s", c)
			return err
		}
	}
	return nil
}
