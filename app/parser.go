package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type Command struct {
	Name         string
	Args         []string
	Redirections []Redirection
	Pipe         string
	isBackground bool
}
type Redirection struct {
	Source      string
	Operator    string
	Destination string
}

const delimiter rune = '\n'

func tokenize(command string) ([]string, error) {
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

	i := 0
	for i < len(runes) {
		r := runes[i]

		if r == '\'' || r == '"' {
			if currentQuote == 0 {
				currentQuote = r
			} else if currentQuote == r {
				currentQuote = 0
			} else {
				token.WriteRune(r)
			}
			i++
			continue
		}

		if currentQuote == 0 {
			if unicode.IsSpace(r) {
				flushToken()
				i++
				continue
			}

			if unicode.IsDigit(r) && token.Len() == 0 {
				token.WriteRune(runes[i])
				for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
					i++
					token.WriteRune(runes[i])
				}
				i++
			}

			switch r {
			case '|':
				flushToken()
				if i+1 < len(runes) && runes[i+1] == '&' {
					tokens = append(tokens, "|&")
					i += 2
				} else {
					tokens = append(tokens, "|")
					i++
				}

			case '&':
				flushToken()
				tokens = append(tokens, "&")
				i++

			case '>':
				if _, err := strconv.Atoi(token.String()); err != nil {
					flushToken()
				}
				token.WriteRune(' ') // for parsing ease
				token.WriteRune('>')

				if i+1 < len(runes) && runes[i+1] == '>' {
					i++
					token.WriteRune(runes[i])
				}
				if i+1 < len(runes) && runes[i+1] == '&' {
					i++
					token.WriteRune(' ')
					token.WriteRune('&')
					for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
						i++
						token.WriteRune(runes[i])
					}
				}
				flushToken()
				i++

			case '<':
				flushToken()
				token.WriteRune(' ')
				token.WriteRune('<')

				if i+1 < len(runes) {
					i++
					if runes[i] == '>' {
						token.WriteRune(runes[i])
					}
					if runes[i] == '&' {
						token.WriteRune(' ')
						token.WriteRune('&')
						for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
							i++
							token.WriteRune(runes[i])
						}
					}
					goto flush
				}

				if i+1 < len(runes) && runes[i+1] == '<' {
					i++
					token.WriteRune(runes[i])
				}
				if i+1 < len(runes) && runes[i+1] == '<' {
					i++
					token.WriteRune(runes[i])
				}
			flush:
				flushToken()
				i++

			case '\\':
				if i+1 < len(runes) {
					next := runes[i+1]
					token.WriteRune(next)
					i++
				} else {
					return tokens, ErrUnexpectedEnd
				}
			default:
				token.WriteRune(r)
			}

		} else { //inside quote
			if r == '\\' {
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
				}
				i++
			} else {
				token.WriteRune(r)
				i++
			}
		}
	}
	flushToken()
	if currentQuote != 0 {
		return tokens, ErrUnexpectedEnd
	}
	return tokens, nil
}

var redirectionOperators = map[string]bool{
	">":   true,
	">>":  true,
	">&":  true,
	">>&": true,

	"<":   true,
	"<<":  true,
	"<&":  true,
	"<<<": true,

	"<>": true,
	"><": true,

	"&>":  true,
	"&>>": true,
}

var redirectionRe = regexp.MustCompile(`^(\d+|&)? (>>|<<|<<<|<>|>|<) (&\d+)?`)

func Parse(command string) ([]*Command, error) {
	var cmds []*Command
	tokens, err := tokenize(command)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%#v\n", tokens)

nextCommand:
	var cmd *Command
	i := 0
	for i < len(tokens) {
		token := tokens[i]

		if token == "&" {
			cmd.isBackground = true
			i++
			continue
		}

		if token == "|" || token == "|&" {
			if cmd == nil {
				return nil, fmt.Errorf("pipe without preceding command")
			}
			cmd.Pipe = token
			cmds = append(cmds, cmd)
			i++
			goto nextCommand
		}

		if matches := redirectionRe.FindStringSubmatch(token); matches != nil {
			// matches[0] full
			// matches[1] source
			// matches[2] Operator
			// matches[3] Destination
			var redirection Redirection
			redirection.Operator = matches[2]

			op := matches[2]
			if op[0] == '>' {
				if matches[1] != "" {
					redirection.Source = matches[1]
				} else {
					redirection.Source = "1"
				}

				if matches[3] != "" {
					redirection.Destination = matches[3][1:]
				} else {
					if i+1 < len(tokens) {
						i++
						redirection.Destination = tokens[i]
					} else {
						return nil, fmt.Errorf("unexpected token `newline` after %s", tokens[i])
					}
				}
			}
			if op[0] == '<' {
				if matches[1] != "" {
					return nil, ErrBadFileDescriptor
				}
				if matches[3] != "" {
					redirection.Source = matches[3][1:]
				} else {
					if i+1 < len(tokens) {
						i++
						redirection.Source = tokens[i]
					} else {
						return nil, fmt.Errorf("unexpected token `newline` after %s", tokens[i])
					}
				}
			}
			cmd.Redirections = append(cmd.Redirections, redirection)
		}

		if cmd.Name == "" {
			cmd.Name = token
		} else {
			cmd.Args = append(cmd.Args, token)
		}
		i++
	}
	cmds = append(cmds, cmd)

	return cmds, nil
}
