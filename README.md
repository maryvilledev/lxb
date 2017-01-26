# LXB - LXD Image Builder

LXB is an attempt to bring a little of the Docker image creation magic to LXD with an automated, templated build file.

Use make to build:
```bash
go get -d github.com/maryvilledev/lxb
cd $GOPATH/src/github.com/maryvilledev/lxb
make
```

## usage
```bash
NAME:
   lxb - LXD Image Builder

USAGE:
   lxb [global options] [arguments...]

VERSION:
   0.1.0

GLOBAL OPTIONS:
   --lxfile, -f "lxfile.yml"  Path to the build spec
   --context, -c "./"   Path to the build context
   --keep, -k     Don't remove the build container when complete
   --verbose      Print extra debugging output
   --remote "local"   LXD daemon address [$LXB_REMOTE]
   --version, -v    print the version
```
The process is similar to building a Docker image. Call `lxb` either in the build context directory (only important if your `lxfile.yml` uses `files` or `templates`), or pass the `-c` flag.

The build config can be supplied by providing nothing (in which case `./lxfile.yml` is loaded), the `--lxfile` flag, or by passing `-` as the first argument (which will read from stdin).

The LXD config is loaded from either the standard location (`~/.config/lxc/config.yml`) or the `LXD_CONF` environment variable, just like LXD itself.

## lxfile usage
Lxfiles are YAML files (just for you @jimmymac) that contain specifications for both the build container and the resulting image.

The only key that is _strictly_ required is `baseimg`, all other keys will be ignored if they are omitted with the exception of `build_networks`. At least one network is required, but Lxb attempts to use defaults if none is specified in the lxfile.

#### Note on templates and files
There are a few gotchas regarding templates and files:

  - *You must run `lxb` as root and use a local daemon if your `lxfile.yml` includes templates.* This is due to the fact that LXD does not support modifying templates through the API, so we've got to modify files on disk under `/var/lib/lxd`. This requires root access. See [this issue](https://github.com/lxc/lxd/issues/1729) for details.
  - Older builds of LXD do not allow files to be manipulated via the API. Lxb attempts to copy files directly in this case too, and so will need to be run against a local daemon as root.

**Example:**

```yaml
# This can be an alias or hash, but must be a local image
baseimg: trusty
image_properties:
  description: Apache2 on Ubuntu Trusty x64
image_aliases:
  - trusty-apache2
public: true
build_networks:
  # Connect to one or more networks during the build
  - lxcbr0
build_profiles:
  - default
build_config:
  # See https://github.com/lxc/lxd/blob/master/doc/configuration.md#container-configuration
  limits.memory: 512MB
files:
  # Use relative paths from the context directory
  # files will be copied with the same permissions (but owned by root)
  - test.txt:/opt/test.txt
templates:
  # Use relative paths from the context directory
  # Templates will be evaluated on create only and no properties will be set
  # See https://github.com/lxc/lxd/blob/master/doc/image-handling.md#content
  - file.tmpl:/dest/file.tmpl
env:
  BUILD_ENV: dev
  CONTAINERIZED: true
  TERM: xterm-color
cmd:
  # A list of commands that will be run inside the container (using /bin/sh)
  - apt-get update
  - apt-get -y install apache2
  - apt-get -y clean
```
