# gitreposerver

[![Go Reference](https://pkg.go.dev/badge/go.seankhliao.com/gitreposerver.svg)](https://pkg.go.dev/go.seankhliao.com/gitreposerver)
[![License](https://img.shields.io/github/license/seankhliao/gitreposerver.svg?style=flat-square)](LICENSE)

Demo of using [go-git](https://github.com/go-git/go-git) as a repo server over ssh and http.

Usage:

```
$ gitreposerver -git-dir ./some/git/repo/.git
2022/07/04 22:40:56 starting http server on :8080
2022/07/04 22:40:56 starting ssh server on :8081
```
