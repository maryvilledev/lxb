package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type BuildSpec struct {
	BaseImg       string            `yaml:"baseimg"`
	ImgAliases    []string          `yaml:"image_aliases,omitempty"`
	ImgProperties map[string]string `yaml:"image_properties,omitempty"`
	Public        bool              `yaml:"public,omitempty"`
	BuildProfiles []string          `yaml:"build_profiles,omitempty"`
	BuildConfig   map[string]string `yaml:"build_config,omitempty"`
	Env           map[string]string `yaml:"env,omitempty"`
	Cmd           []string          `yaml:"cmd,omitempty"`
	Files         []string          `yaml:"files,omitempty"`
}

// LoadBuildSpec takes a string argument that is either a path to a YML file
// or the raw YML itself and returns a BuildSpec
func LoadBuildSpec(yml string) *BuildSpec {
	b := new(BuildSpec)
	contents := []byte(yml)
	if fileExists(yml) {
		contents = asrt(ioutil.ReadFile(yml)).([]byte)
	}

	if err := yaml.Unmarshal(contents, b); err != nil {
		panic("Failed to parse LXfile")
	}
	return b
}
