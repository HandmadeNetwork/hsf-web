# Handmade Software Foundation Web Template

This codebase is a simple Go application for serving a website. It is synthesized from years of development on the [Handmade Network website](https://github.com/HandmadeNetwork/hmn).

In keeping with many Handmade projects, this code is intended to serve as a starting point, _not_ as a complete, batteries-included framework. You are encouraged to copy-paste this code into your own codebase and take ownership of it for yourself, without expecting to transparently upgrade in the future. We like it and have found it to be well-designed; we hope you do too.

## Getting started

First create a `config.go` file by copying `src/config/config.go.example`:

```
# On Windows
copy src\config\config.go.example src\config\config.go

# On Mac and Linux
cp src/config/config.go.example src/config/config.go
```

Then simply run it:

```
go run .
```

## Why Go?

We consider Go to be an all-around good choice for web development. The language runtime handles concurrency effectively, it is easy to avoid allocations when necessary, it is reasonably fast both to run and compile, it has a large and generally high-quality standard library, and most importantly, the tooling is best-in-class.

The Go tooling is trivial to install and works well on all platforms. It produces statically-linked, standalone binaries that are trivial to deploy on any server. It can cross-compile effortlessly, and there is no need for a build system or awful set of config files.

Overall it is a very pragmatic language and we think few programmers will be disappointed with it for web development.

## Why server-rendered instead of a static site?

We find that most websites want to be dynamic in some way. Eventually every site demands a newsletter signup, a documentation search, or some kind of background task. While you can indeed reduce latency with a static site or SPA, we appreciate the simplicity and reliability of just running a program on a server, where we can ensure that the program is fast. (You can always add a cache for the static parts.)
