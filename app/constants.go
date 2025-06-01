package main

import "errors"

const delimiter rune = '\n'

var builtins = map[string]struct{}{
	"exit":    {},
	"echo":    {},
	"type":    {},
	"pwd":     {},
	"cd":      {},
	"history": {},
}

var redirectionOperators = map[string]struct{}{
	"1>": {},
	"2>": {},
	"&>": {},
}

var ParseErrNoCommand = errors.New("no command")
var ParseErrNoTrailingQuote = errors.New("no trailing quote")
var ErrTooManyArguments = errors.New("too many arguments")
var ErrNoRedirectionFile = errors.New("no file after redirection")
