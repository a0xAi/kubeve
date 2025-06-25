# `kubeve`: Human way to look into Kubernetes Events

![Latest GitHub release](https://img.shields.io/github/v/release/a0xAi/kubeve.svg)
![GitHub stars](https://img.shields.io/github/stars/a0xAi/kubeve.svg?label=github%20stars)

## What is `kubeve`?

**kubeve** is a tool to for reading Kubernetes events in human way<br/>

![kubeve demo PNG](img/kubeve.png)

## Installation
### Brew 
`brew tap a0xAi/kubeve`<br>
`brew install kubeve`

### Manual Installation (macOS and Linux)
- Download `kubeve` binary from releases..
- Either:
  - save it to somewhere in your PATH,
  - or save them to a directory, then create symlinks to `kubeve` from somewhere in your PATH, like /usr/local/bin
- Make `kubeve` executable (chmod +x ...)

## Configuration

`kubeve` looks for a YAML configuration file at `~/.kubeve/config.yaml` on start. If the file is not present, built in defaults are used.

Example default configuration:

```yaml
config:
  flags:
    disableLogo: false
  theme:
    backgroundColor: '#0000ff'
    textColor: '#00ff00'
```