package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

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
					} else {
						return tokens, ErrUnexpectedEnd
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
	isBackground bool
}

type ParsedInputSequence struct {
	ParsedCommands []ParsedCommand
	Operators      []string
}

var redirectAttemptRe = regexp.MustCompile(`^(\d+)?(>|<)`)
var redirectionRe = regexp.MustCompile(`^(\d+)?(>>|<<-?|<<<|<&-?|>&-?|>|<|<>)(\d+)?`)

func (s *Shell) ParseInput(command string) (*ParsedInputSequence, error) {
	tokens, err := tokenize(command)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(s.term,"%#v\n", tokens)

	sequence := &ParsedInputSequence{
		ParsedCommands: []ParsedCommand{},
		Operators:      []string{},
	}

	parsedCmd := ParsedCommand{}
	i := 0
	for i < len(tokens) {
		token := tokens[i]

		switch token {
		case "&":
			parsedCmd.isBackground = true

		case "|", "|&", "&&", "||", ";":
			if parsedCmd.Name == "" && len(parsedCmd.Redirections) == 0 {
				return nil, fmt.Errorf("%s without preceding command", token)
			}
			sequence.ParsedCommands = append(sequence.ParsedCommands, parsedCmd)
			sequence.Operators = append(sequence.Operators, token)

			parsedCmd = ParsedCommand{}

		default:
			if matches := redirectionRe.FindStringSubmatch(token); matches != nil {
				var nextToken string
				// next token == "" if dont need next token
				if matches[3] != "" || matches[2] == ">&-" || matches[2] == "<&-" {
					nextToken = ""
				} else {
					if i+1 >= len(tokens) {
						return nil, fmt.Errorf("syntax error: unexpected token `newline` after %s", token)
					}

					if isOperator(tokens[i+1]) {
						return nil, fmt.Errorf("syntax error: unexpected token `%s` after %s", nextToken, matches[2])
					}
					nextToken = tokens[i+1]
					i++
				}

				redirection, err := s.getRedirection(matches, nextToken)
				if err != nil {
					return nil, err
				}
				parsedCmd.Redirections = append(parsedCmd.Redirections, redirection)

			} else {
				if parsedCmd.Name == "" {
					parsedCmd.Name = token
				} else {
					parsedCmd.Args = append(parsedCmd.Args, token)
				}
			}
		}
		i++
	}

	if parsedCmd.Name != "" || len(parsedCmd.Redirections) > 0 {
		sequence.ParsedCommands = append(sequence.ParsedCommands, parsedCmd)
	}

	if len(sequence.Operators) != len(sequence.ParsedCommands)-1 {
		return nil, ErrUnexpectedEnd
	}

	return sequence, nil
}

func isOperator(token string) bool {
	switch token {
	case "|", "|&", "&&", "||", ";", "&":
		return true
	default:
		return false
	}
}

func (s *Shell) getRedirection(matches []string, nextToken string) (Redirection, error) {
	// matches[0] full
	// matches[2] Operator
	// matches[1] source, matches[3] Destination
	op := matches[2]

	switch op {
	case ">", ">>":
		sourceFD := 1
		if matches[1] != "" {
			fd, _ := strconv.Atoi(matches[1])
			sourceFD = fd
		}

		return &OutputRedirection{
			Operator:   op,
			SourceFD:   sourceFD,
			TargetFile: nextToken,
			TargetFD:   nil,
		}, nil

	case ">&":
		sourceFD := 1
		if matches[1] != "" {
			fd, _ := strconv.Atoi(matches[1])
			sourceFD = fd
		}

		targetFD, _ := strconv.Atoi(matches[3])

		return &OutputRedirection{
			Operator:   op,
			SourceFD:   sourceFD,
			TargetFile: "",
			TargetFD:   &targetFD,
		}, nil

	case ">&-":
		targetFD := 1
		if matches[1] != "" {
			fd, _ := strconv.Atoi(matches[1])
			targetFD = fd
		}

		return &RedirectionCloser{
			Operator: op,
			TargetFD: targetFD,
		}, nil

	case "<&-":
		targetFD := 0
		if matches[1] != "" {
			fd, _ := strconv.Atoi(matches[1])
			targetFD = fd
		}

		return &RedirectionCloser{
			Operator: op,
			TargetFD: targetFD,
		}, nil

	case "<":
		targetFD := 0
		if matches[1] != "" {
			targetFD, _ = strconv.Atoi(matches[1])
		}

		return &InputRedirection{
			Operator:   op,
			TargetFD:   targetFD,
			SourceFile: nextToken,
			SourceFD:   nil,
		}, nil

	case "<&":
		targetFD := 0
		if matches[1] != "" {
			targetFD, _ = strconv.Atoi(matches[1])
		}

		sourceFD, _ := strconv.Atoi(nextToken)
		return &InputRedirection{
			Operator:   op,
			TargetFD:   targetFD,
			SourceFile: "",
			SourceFD:   &sourceFD,
		}, nil

	case "<<", "<<-":
		targetFD := 0
		if matches[1] != "" {
			targetFD, _ = strconv.Atoi(matches[1])
		}

		delimiter := nextToken
		var content strings.Builder
		s.term.SetPrompt(promptNextLine)
		for {
			line, err := s.term.ReadLine()
			if err != nil {
				return nil, fmt.Errorf("Error reading line '%s'", delimiter)
			}

			if line == delimiter {
				break
			}
			content.WriteString(line)
			content.WriteString("\n")
		}
		s.term.SetPrompt(promptDefault)

		return &HereRedirection{
			Operator: op,
			TargetFD: targetFD,
			Content:  content.String(),
		}, nil

	case "<<<":
		targetFD := 0
		if matches[1] != "" {
			targetFD, _ = strconv.Atoi(matches[1])
		}

		content := nextToken
		return &HereRedirection{
			Operator: op,
			TargetFD: targetFD,
			Content:  content,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported operator: %s", op)
	}
}

type Redirection interface {
	GetType() string
}

type OutputRedirection struct {
	Operator   string
	SourceFD   int
	TargetFile string
	TargetFD   *int
}

func (r *OutputRedirection) GetType() string { return "output" }

type InputRedirection struct {
	Operator   string
	TargetFD   int
	SourceFile string
	SourceFD   *int
}

func (r *InputRedirection) GetType() string { return "input" }

type RedirectionCloser struct {
	Operator string
	TargetFD int
}

func (r *RedirectionCloser) GetType() string { return "closer" }

type HereRedirection struct {
	Operator string
	TargetFD int
	Content  string
}

func (r *HereRedirection) GetType() string { return "here" }
