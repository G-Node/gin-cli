# GIN-cli

[![Build Status](https://travis-ci.org/G-Node/gin-cli.svg?branch=master)](https://travis-ci.org/G-Node/gin-cli) [![Coverage Status](https://coveralls.io/repos/github/G-Node/gin-cli/badge.svg?branch=master)](https://coveralls.io/github/G-Node/gin-cli?branch=master) [![GoDoc](https://godoc.org/github.com/G-Node/gin-cli?status.svg)](http://godoc.org/github.com/G-Node/gin-cli)

**G**-Node **In**frastructure command line client

This package is a command line client for interfacing with the [gin-auth](https://github.com/G-Node/gin-auth) and [gin-repo](https://github.com/G-Node/gin-repo) services.
It offers a simplified interface for downloading and uploading files from repositories hosted on Gin.

## Usage

The following is a description of the available commands in the GIN client.
In the command line, you can view a basic list of commands by running

    gin -h

You can also run

    gin help <cmd>

to get the full description of any command:

| command | arguments | description |
| ------- | --------- | ----------- |
| login   | username  | Login to the GIN services |
|         |           | If no username is specified on the command line, it will 
