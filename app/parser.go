package main

import (
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

	s := newRuneStack()
	var token strings.Builder

	flushToken := func() {
		if token.Len() > 0 {
			tokens = append(tokens, token.String())
			token.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if s.isEmpty() && unicode.IsSpace(r) && r != delimiter {
			flushToken()
			continue
		}

		if r == '\'' || r == '"' {
			if s.quote == r {
				s = newRuneStack()
			} else if s.top() == r {
				if s.quote != r {
					token.WriteRune(r)
				}
				s.pop(r)
			} else {
				s.push(r)
				if s.quote != r {
					token.WriteRune(r)
				}
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
			if s.quote == '"' {
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
			} else if s.quote == '\'' {
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

	command = strings.TrimSpace(command)
	if command == "" {
		return cmd, nil, ParseErrNoCommand
	}

	tokens := tokenize(command)
	// fmt.Printf("%#v\n", tokens)

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

type runeStack struct {
	stack []rune
	quote rune
}

func (s *runeStack) top() rune {
	if len(s.stack) == 0 {
		return 0
	}
	return s.stack[len(s.stack)-1]
}
func (s *runeStack) pop(r rune) {
	if len(s.stack) > 0 && s.stack[len(s.stack)-1] == r {
		s.stack = s.stack[:len(s.stack)-1]
	}
	if len(s.stack) == 0 {
		s.quote = 0
	}
}
func (s *runeStack) push(r rune) {
	if len(s.stack) == 0 {
		s.quote = r
	}
	s.stack = append(s.stack, r)
}
func (s *runeStack) isEmpty() bool {
	return len(s.stack) == 0
}

func newRuneStack() runeStack {
	stack := make([]rune, 0, 2)
	return runeStack{
		stack: stack,
	}
}
