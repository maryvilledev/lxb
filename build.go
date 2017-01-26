package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lxc/lxd"
	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	log "github.com/sirupsen/logrus"
)

// DirectoryManipulation is the const for LXD's `directory_manipulation` API extension
const DirectoryManipulation = "directory_manipulation"

// Build represents the entirety of the build job
type Build struct {
	ID, Remote, imgID string
	spec              *BuildSpec
	client            *lxd.Client
}

// NewBuild generates a new Build
func NewBuild(spec *BuildSpec, client *lxd.Client, remote string) *Build {
	return &Build{
		spec:   spec,
		client: client,
		ID:     spec.BaseImg + "-build-" + fmt.Sprintf("%v", time.Now().Unix()),
		Remote: remote,
	}
}

// Execute runs commands in a container
func (b *Build) Execute(keepContainer bool) error {
	farray := []func() error{
		b.createBuildContainer,
		b.startBuildContainer,
		b.copyFiles,
		b.addTemplates,
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

	// If we have no BuildNetworks defined in the lxfile
	// try some defaults
	if len(b.spec.BuildNetworks) < 1 {
		b.spec.BuildNetworks = []string{}
		for _, n := range []string{"default", "lxcbr0"} {
			_, err := b.client.NetworkGet(n)
			if err != nil {
				continue
			}
			b.spec.BuildNetworks = append(b.spec.BuildNetworks, n)
			break
		}
	}

	if len(b.spec.BuildNetworks) < 1 {
		return fmt.Errorf("No valid networks found! Please specify one in your lxfile.")
	}

	if b.spec.Devices == nil {
		b.spec.Devices = map[string]map[string]string{}
	}

	for _, net := range b.spec.BuildNetworks {
		network, err := b.client.NetworkGet(net)
		if err != nil {
			log.Debug(err)
			continue
		}

		if network.Type == "bridge" {
			b.spec.Devices[net] = map[string]string{"type": "nic", "nictype": "bridged", "parent": net}
		} else {
			b.spec.Devices[net] = map[string]string{"type": "nic", "nictype": "macvlan", "parent": net}
		}
	}

	resp, err := b.client.Init(
		b.ID,
		b.Remote,
		b.spec.BaseImg,
		&b.spec.BuildProfiles,
		b.spec.BuildConfig,
		b.spec.Devices,
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
		resp *api.Response
	)

	if resp, err = b.client.Action(b.ID, shared.Start, 30, false, false); err != nil {
		log.Debugln("Failed during startBuildContainer")
		return err
	}

	if err = b.client.WaitForSuccess(resp.Operation); err != nil {
		log.Debugln("Failed during startBuildContainer")
		return err
	}

	b.client.NetworkPut("default", api.NetworkPut{Config: map[string]string{}})

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
	b.imgID, err = b.client.ImageFromContainer(
		b.ID,
		b.spec.Public,
		b.spec.ImgAliases,
		b.spec.ImgProperties,
		b.spec.CompressionAlgo,
	)
	if err == nil {
		log.Infoln("Created image", b.imgID)
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

func (b *Build) addTemplates() error {
	if len(b.spec.Templates) > 0 {
		log.Infoln("Adding templates to container")
		containerBaseDir := "/var/lib/lxd/containers/" + b.ID
		metaFile := containerBaseDir + "/metadata.yaml"

		if !dirExists(containerBaseDir) {
			return fmt.Errorf("Directory %s does not exist!", containerBaseDir)
		}

		if !fileExists(metaFile) {
			return fmt.Errorf("Metadata file %s does not exist!", metaFile)
		}

		f, err := os.Open(metaFile)
		if err != nil {
			return err
		}

		contents, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		var metaInterface map[string]interface{}
		if err = json.Unmarshal(contents, &metaInterface); err != nil {
			return err
		}

		tmpl := metaInterface["templates"].(map[string]interface{})
		for _, t := range b.spec.Templates {
			split := strings.SplitN(t, ":", 2)
			srcFile := split[0]
			destFile := split[1]

			log.Debugf("Template %s will be placed at %s", srcFile, destFile)
			tmplEntry := struct {
				Template string   `json:"template"`
				When     []string `json:"when"`
			}{
				Template: srcFile,
				When:     []string{"create"},
			}

			tmpl[destFile] = tmplEntry
			tmplFilePath := containerBaseDir + "/templates/" + filepath.Base(srcFile)
			log.Debugf("Copying %s to %s", srcFile, tmplFilePath)
			if _, err := Copy(srcFile, tmplFilePath); err != nil {
				return err
			}
		}

		result, err := json.Marshal(metaInterface)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(metaFile, result, 0644)
		return err
	}
	return nil
}

func (b *Build) copyFiles() error {
	var err error
	if len(b.spec.Files) < 1 {
		return err
	}

	if hasExtension(b.client, DirectoryManipulation) {
		err = b.apiCopyFiles()
	} else {
		err = b.manualCopyFiles()
	}
	return err
}

func (b *Build) apiCopyFiles() error {
	for _, fileString := range b.spec.Files {
		contextPath, containerPath, err := splitFilePath(fileString)
		if err != nil {
			return err
		}

		fi, err := os.Stat(contextPath)
		if err != nil {
			return err
		}

		// Push recursively if this is a dir
		if fi.IsDir() {
			if err = b.client.RecursivePushFile(b.ID, contextPath, containerPath); err != nil {
				return err
			}
			continue
		}

		// Otherwise push normally
		file, err := os.Open(contextPath)
		if err != nil {
			return err
		}
		if err = b.client.PushFile(b.ID, containerPath, -1, -1, fi.Mode().String(), file); err != nil {
			return err
		}
	}
	return nil
}

func (b *Build) manualCopyFiles() error {
	var err error
	log.Infoln("Pushing files into container")
	for _, fileString := range b.spec.Files {
		var (
			fInfo os.FileInfo
			f     *os.File
		)
		contextPath, containerPath, err := splitFilePath(fileString)
		if err != nil {
			log.Warn(err)
			continue
		}

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
				if err = b.client.PushFile(b.ID, containerPath, 0, 0, fInfo.Mode().String(), f); err != nil {
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
			if err = b.client.PushFile(b.ID, containerPath, 0, 0, fInfo.Mode().String(), f); err != nil {
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
		if code, err := b.client.Exec(b.ID,
			[]string{"/bin/sh", "-c", c},
			b.spec.Env,
			nil,
			os.Stdout,
			os.Stderr,
			func(l *lxd.Client, w *websocket.Conn) {},
			80, 24); err != nil || code != 0 {
			return fmt.Errorf("Failed during build command %s: Exit code %v Error: %v", c, code, err)
		}
	}
	return nil
}
