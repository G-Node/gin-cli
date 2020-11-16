# GIN-CLI

[![GoDoc](https://godoc.org/github.com/G-Node/gin-cli?status.svg)](http://godoc.org/github.com/G-Node/gin-cli)

[![Build Status](https://travis-ci.org/G-Node/gin-cli.svg?branch=master)](https://travis-ci.org/G-Node/gin-cli)
[![Build status](https://ci.appveyor.com/api/projects/status/gu9peb10f9k8ed3d/branch/master?svg=true)](https://ci.appveyor.com/project/G-Node/gin-cli/branch/master)

---

**G**-Node **In**frastructure **C**ommand **L**ine **C**lient

This package is a command line client for interfacing with repositories hosted on [GIN](https://gin.g-node.org).
It offers a simplified interface for downloading and uploading files from repositories hosted on GIN.

It consists of commands for interfacing with the GIN web API (e.g., listing repositories, creating repositories, managing SSH keys) but primarily, it wraps **git** and **git-annex** commands to make working with data repositories easier.

## Information, setup, and guides
For installation instructions see the [GIN Client Setup](https://gin.g-node.org/G-Node/Info/wiki/GIN+CLI+Setup) page.

General information, help, and guides for using GIN can be found on the [GIN Info Wiki](https://gin.g-node.org/G-Node/info/wiki).
Help and information for the client in particular can be on the following pages:
- [Usage guide (tutorial)](https://gin.g-node.org/G-Node/Info/wiki/GIN+CLI+Usage+Tutorial)
- [Useful recipes and short workflows](https://gin.g-node.org/G-Node/Info/wiki/GIN+CLI+Recipes)
- [Detailed command overview](https://gin.g-node.org/G-Node/Info/wiki/GIN+CLI+Help)
