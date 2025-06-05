package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

type Command struct {
	Name    string
	Options []string
	Args    []string
}
type Redirection struct {
	Operator string
	File     string
}

func tokenize(command string) []string {
	runes := []rune(command)
	var tokens []string

	var currentQuote rune = 0
	var token strings.Builder

	flushToken := func() {
		if token.Len() > 0 {
			tokens = append(tokens, token.String())
			token.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if currentQuote == 0 && unicode.IsSpace(r) && r != delimiter {
			flushToken()
			continue
		}

		if r == '\'' || r == '"' {

			if currentQuote == 0 {
				currentQuote = r
			} else if currentQuote == r {
				currentQuote = 0
			} else {
				token.WriteRune(r)
			}
			continue
		}

		switch r {
		case '|':
			flushToken()
			tokens = append(tokens, "|")

		case '&':
			flushToken()
			token.WriteRune('&')

		case '1', '2':
			if i+1 < len(runes) && (runes[i+1] == '<' || runes[i+1] == '>') {
				flushToken()
			}
			token.WriteRune(r)

		case '<', '>':
			switch token.String() {
			case "&":
				token.WriteRune(r)
				flushToken()
				continue
			case "1", "2":
				token.WriteRune(r)
			default:
				flushToken()
				token.WriteRune(r)
			}
			if i+1 < len(runes) && runes[i+1] == '>' {
				i++
				token.WriteRune(runes[i])
			}
			flushToken()

		case '\\':
			if currentQuote == '"' {
				if i+1 < len(runes) {
					i++
					next := runes[i]
					switch next {
					case '\n':
						token.WriteRune('\n')
					case '\\':
						token.WriteRune('\\')
					case '$':
						token.WriteRune('$')
					case '"':
						token.WriteRune('"')
					default:
						token.WriteRune('\\')
						token.WriteRune(next)
					}
				}
			} else if currentQuote == '\'' {
				token.WriteRune('\\')
			} else {
				if i+1 < len(runes) {
					next := runes[i+1]
					token.WriteRune(next)
					i++
				}
			}

		default:
			token.WriteRune(r)
		}
	}

	flushToken()
	return tokens
}

func isRedirection(r string) bool {
	return slices.Contains([]string{"1>", "2>", "&>", ">", ">>", "1>>", "2>>", "1<", "2<", "&<"}, r)
}

func ParseCommand(command string) (Command, []Redirection, error) {
	var cmd Command
	var redirections []Redirection

	tokens := tokenize(command)
	fmt.Printf("%#v\n", tokens)

	i := 0
	cmdName := tokens[0]
	cmd.Name = cmdName
	i = 1

	// arguments
	var args []string
	for i < len(tokens) {
		if isRedirection(tokens[i]) {
			operator := tokens[i]
			if i+1 < len(tokens) {
				i++
				if isRedirection(tokens[i]) {
					return cmd, nil, ErrNoRedirectionFile
				}
				redirection := Redirection{
					Operator: operator,
					File:     tokens[i],
				}
				redirections = append(redirections, redirection)
			} else {
				return cmd, nil, ErrNoRedirectionFile
			}
			i++
			continue
		}

		arg := tokens[i]
		args = append(args, arg)
		i++
	}
	cmd.Args = args

	return cmd, redirections, nil
}
