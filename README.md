# Expressive flags for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/cavaliergopher/xflags.svg)](https://pkg.go.dev/github.com/cavaliergopher/xflags) [![Build Status](https://app.travis-ci.com/cavaliergopher/xflags.svg?branch=main)](https://app.travis-ci.com/cavaliergopher/xflags)

Package xflags implements command-line flag parsing and is a compatible
alternative to Go's flag package. This package provides higher-order features
such as subcommands, positional arguments, required arguments, validation,
support for environment variables and others.

Package xflags aims to make composing large, full-featured command line tools as
simple and clean as possible. The Builder pattern is employed with method
chaining to configure commands and flags declaratively with error checking.
