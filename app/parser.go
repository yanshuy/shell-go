package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

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
				token.WriteRune('>')

				if i+1 < len(runes) {
					switch runes[i+1] {
					case '&':
						i++
						token.WriteRune('&')

						if i+1 < len(runes) {
							if runes[i+1] == '-' {
								i++
								token.WriteRune(runes[i])
							} else {
								for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
									i++
									token.WriteRune(runes[i])
								}
							}
						}
					case '>':
						i++
						token.WriteRune(runes[i])
					}
				}
				flushToken()
				i++

			case '<':
				if _, err := strconv.Atoi(token.String()); err != nil {
					flushToken()
				}
				token.WriteRune('<')

				if i+1 < len(runes) {
					switch runes[i+1] {
					case '>':
						i++
						token.WriteRune(runes[i])

					case '&':
						i++
						token.WriteRune('&')

						if i+1 < len(runes) {
							if runes[i+1] == '-' {
								i++
								token.WriteRune(runes[i])
							} else {
								for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
									i++
									token.WriteRune(runes[i])
								}
							}
						}

					case '<':
						i++
						token.WriteRune(runes[i])

						if i+1 < len(runes) && (runes[i+1] == '-' || runes[i+1] == '<') {
							i++
							token.WriteRune(runes[i])
						}

					}
				}
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

type ParsedCommand struct {
	Name         string
	Args         []string
	Redirections []Redirection
	PipeOperator string
	isBackground bool
}

type Redirection struct {
	Type        string
	Source      any
	Destination any
	Content     string
}

var redirectAttemptRe = regexp.MustCompile(`^(\d+)?(>|<)`)
var redirectionRe = regexp.MustCompile(`^(\d+|&)?(>>|<<-?|<<<|<&|>&|>|<|<>)(\d+)?`)

func (s *Shell) ParseInput(command string) ([]*ParsedCommand, error) {
	var parsedCmds []*ParsedCommand
	tokens, err := tokenize(command)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%#v\n", tokens)

	parsedCmd := &ParsedCommand{}
	i := 0
	for i < len(tokens) {
		token := tokens[i]

		if token == "&" {
			parsedCmd.isBackground = true
			i++
			continue
		}

		if token == "|" || token == "|&" {
			if parsedCmd.Name == "" {
				return nil, fmt.Errorf("pipe without preceding command")
			}
			parsedCmd.PipeOperator = token
			parsedCmds = append(parsedCmds, parsedCmd)
			parsedCmd = &ParsedCommand{}
			i++
		}

		if redirectAttemptRe.MatchString(token) {
			matches := redirectionRe.FindStringSubmatch(token)
			// matches[0] full
			// matches[2] Operator
			// matches[1] source, matches[3] Destination

			var redirection Redirection
			redirection.Type = matches[2]

			op := matches[2]
			switch op {
			case ">", ">>":
				if matches[1] == "&" {
					redirection.Source = matches[1]
				} else if matches[1] != "" {
					fd, _ := strconv.Atoi(matches[1])
					redirection.Source = fd
				} else {
					redirection.Source = 1
				}

				if i+1 < len(tokens) {
					i++
					redirection.Destination = tokens[i]
				} else {
					return nil, fmt.Errorf("syntax error no file after %s", op)
				}

			case ">&":
				if matches[1] == "&" {
					redirection.Source = matches[1]
				} else if matches[1] != "" {
					fd, _ := strconv.Atoi(matches[1])
					redirection.Source = fd
				} else {
					redirection.Source = 1
				}

				dest := matches[3]
				if dest == "" {
					return nil, fmt.Errorf("syntax error no FD after &")
				} else if dest == "-" {
					redirection.Destination = -1
				} else {
					fd, _ := strconv.Atoi(dest)
					redirection.Destination = fd
				}

			case "<":
				if matches[1] != "" {
					fd, _ := strconv.Atoi(matches[1])
					if fd == 1 || fd == 2 {
						return nil, ErrBadFileDescriptor
					}
					redirection.Destination = fd
				} else {
					redirection.Destination = 0
				}

				if i+1 < len(tokens) {
					i++
					redirection.Source = tokens[i]
				} else {
					return nil, fmt.Errorf("syntax error unexpected token `newline` after %s", tokens[i])
				}

			case "<&":
				if matches[1] != "" {
					fd, _ := strconv.Atoi(matches[1])
					if fd == 1 || fd == 2 {
						return nil, ErrBadFileDescriptor
					}
					redirection.Destination = fd
				} else {
					redirection.Destination = 0
				}

				dest := matches[3][1:]
				if dest == "-" {
					redirection.Source = -1
				} else {
					fd, _ := strconv.Atoi(dest)
					redirection.Source = fd
				}

			case "<<":

			}
			parsedCmd.Redirections = append(parsedCmd.Redirections, redirection)

		}

		if parsedCmd.Name == "" {
			parsedCmd.Name = token
		} else {
			parsedCmd.Args = append(parsedCmd.Args, token)
		}
		i++
	}
	parsedCmds = append(parsedCmds, parsedCmd)

	return parsedCmds, nil
}
