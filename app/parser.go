package main

import (
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

func ParseCommand(command string) (Command, []Redirection, error) {
	var cmd Command
	var redirects []Redirection

	command = strings.TrimSpace(command)
	if command == "" {
		return cmd, nil, ParseErrNoCommand
	}

	runes := []rune(command)
	runes = append(runes, delimiter)

	i := 0
	cmdName, newPos, err := parseToken(runes, i, []rune{'>'})
	if err != nil {
		return cmd, nil, err
	}
	if cmdName == "" {
		return cmd, nil, ParseErrNoCommand
	}

	cmd.Name = cmdName
	i = newPos

	for unicode.IsSpace(runes[i]) && runes[i] != delimiter {
		i++
	}

	// arguments
	var args []string
	for runes[i] != delimiter {
		if runes[i] == '>' {
			break
		}

		arg, newPos, err := parseToken(runes, i, []rune{'>'})
		if err != nil {
			return cmd, nil, err
		}
		if arg != "" {
			args = append(args, arg)
		}
		i = newPos

		if unicode.IsSpace(runes[i]) && runes[i] != delimiter {
			i++
		}
	}
	// fmt.Printf("before redirection %#v\n", args)

	// redirection
	for runes[i] == '>' {
		var operator = ">"

		if len(args) > 0 {
			lastArg := args[len(args)-1]
			if len(lastArg) > 0 {
				lastChar := lastArg[len(lastArg)-1]
				switch lastChar {
				case '1':
					operator = "1>"
				case '2':
					operator = "2>"
				case '&':
					operator = "&>"
				}
				if operator != ">" {
					if len(lastArg) == 1 {
						args = args[:len(args)-1]
					} else {
						args[len(args)-1] = lastArg[:len(lastArg)-1]
					}
				}
			}
		}
		i++

		for unicode.IsSpace(runes[i]) && runes[i] != delimiter {
			i++
		}

		if runes[i] == delimiter {
			return cmd, nil, ErrNoRedirectionFile
		}

		filePath, newPos, err := parseToken(runes, i, []rune{'>'})
		if err != nil {
			return cmd, nil, err
		}
		if filePath == "" {
			return cmd, nil, ErrNoRedirectionFile
		}

		redirection := Redirection{
			Operator: operator,
			File:     filePath,
		}
		redirects = append(redirects, redirection)
		i = newPos

		for unicode.IsSpace(runes[i]) && runes[i] != delimiter {
			i++
		}
	}
	// fmt.Printf("%#v", args)

	if runes[i] != delimiter {
		return cmd, redirects, ErrArgsAfterRedirection
	}

	cmd.Args = args
	return cmd, redirects, nil
}

// handling quotes
func parseToken(runes []rune, start int, stopChars []rune) (string, int, error) {
	s := newRuneStack()

	var token strings.Builder
	i := start

	for runes[i] != delimiter {
		current := runes[i]

		if s.isEmpty() {
			if unicode.IsSpace(current) {
				break
			}
			for _, stopChar := range stopChars {
				if current == stopChar {
					goto done // Break out of both loops
				}
			}
		}

		if current == '\'' {
			if s.top() == '\'' {
				s.pop()
			} else if s.top() == '"' {
				token.WriteRune('\'')
			} else {
				s.push('\'')
			}
			i++
			continue
		}

		if current == '"' {
			if s.top() == '"' {
				s.pop()
			} else if s.top() == '\'' {
				token.WriteRune('"')
			} else {
				s.push('"')
			}
			i++
			continue
		}

		if current == '\\' {
			if s.top() == '"' {
				if runes[i+1] != delimiter {
					next := runes[i+1]
					switch next {
					case 'n':
						token.WriteRune('\\')
						token.WriteRune('n')
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

					i += 2
					continue
				}
			} else if s.top() == '\'' {
				token.WriteRune('\\')
				i++
				continue
			} else {
				if runes[i+1] != delimiter {
					next := runes[i+1]
					token.WriteRune(next)
					i += 2
					continue
				}
			}
		}

		token.WriteRune(current)
		i++
	}

done:
	if !s.isEmpty() {
		return "", i, ParseErrNoTrailingQuote
	}

	return token.String(), i, nil
}

type runeStack struct {
	stack []rune
}

func (rs *runeStack) top() rune {
	if rs.isEmpty() {
		return 0
	}
	return rs.stack[len(rs.stack)-1]
}
func (rs *runeStack) pop() {
	if len(rs.stack) > 0 {
		rs.stack = rs.stack[:len(rs.stack)-1]
	}
}
func (rs *runeStack) push(r rune) {
	rs.stack = append(rs.stack, r)
}
func (rs *runeStack) isEmpty() bool {
	return len(rs.stack) == 0
}
func newRuneStack() runeStack {
	stack := make([]rune, 0, 2)
	return runeStack{
		stack: stack,
	}
}
