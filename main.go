package main

import (
	"github.com/codegangsta/cli"
	"github.com/lxc/lxd"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	app := cli.NewApp()
	app.Name = "lxb"
	app.HelpName = "lxb"
	app.Version = "0.0.1"
	app.HideHelp = true
	app.HideVersion = true
	app.Usage = "LXD Image Builder"
	app.ArgsUsage = ""
	app.Action = build
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "lxfile,f",
			Usage: "Path to the build spec",
			Value: "lxfile.yml",
		},
		cli.StringFlag{
			Name:  "context,c",
			Usage: "Path to the build context",
			Value: "./",
		},
		cli.BoolFlag{
			Name:  "keep,k",
			Usage: "Don't remove the build container when complete",
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Print extra debugging output",
		},
		cli.StringFlag{
			Name:   "remote",
			Usage:  "LXD daemon address",
			Value:  "local",
			EnvVar: "LXB_REMOTE",
		},
	}
	app.Run(os.Args)
}

func build(c *cli.Context) {
	// Validate args
	if c.GlobalBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	// Load the build spec
	lxfile := c.String("lxfile")
	buildContext := c.String("context")
	if !fileExists(lxfile) || c.Args().First() == "-" {
		b, err := ioutil.ReadAll(os.Stdin)
		lxfile = string(b)
		if err != nil {
			log.Error(err)
		}
	}
	spec := LoadBuildSpec(lxfile)
	log.Debugln("Loaded build spec")

	// validate the context dir
	contextAbsPath := asrt(filepath.Abs(buildContext)).(string)
	if !dirExists(contextAbsPath) {
		log.Errorln("Build context directory does not exist!")
	}

	// move to context dir
	if err := os.Chdir(contextAbsPath); err != nil {
		log.Error(err)
	}

	// connect to LXD
	cl, err := lxd.NewClient(&lxd.DefaultConfig, c.GlobalString("remote"))
	if err != nil {
		log.Error(err)
	}
	if !cl.AmTrusted() {
		log.Errorln("Connection is not trusted by LXD! Check your cert and key.")
	}
	log.Debugln("Connected to LXD")

	// create build object
	b := NewBuild(spec, cl, c.GlobalString("remote"))
	log.Infoln("Starting build")
	if err := b.Execute(c.Bool("keep")); err != nil {
		log.Error(err)
	}

	log.Println("Build complete!")
}
