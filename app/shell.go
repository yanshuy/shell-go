package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
)

type Shell struct {
	term       *term.Terminal
	currentDir string
	builtins   map[string]builtinCmd
}

type builtinCmd func([]string, IOFiles) int

func NewShell(doneChan <-chan bool) *Shell {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		fmt.Println("Stdin is not a terminal")
		os.Exit(1)
	}

	prevState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Println("Error setting raw mode:", err)
		os.Exit(1)
	}

	go func() {
		<-doneChan
		term.Restore(int(os.Stdin.Fd()), prevState)
	}()

	t := term.NewTerminal(os.Stdin, "$ ")

	shell := &Shell{
		term:       t,
		currentDir: ".",
	}

	builtins := map[string]builtinCmd{
		"exit":    shell.ExitCmd,
		"echo":    shell.EchoCmd,
		"type":    shell.TypeCmd,
		"pwd":     shell.PwdCmd,
		"cd":      shell.CdCmd,
		"env":     shell.envCmd,
		"history": shell.HistoryCmd,
	}

	shell.builtins = builtins

	return shell
}

func (s *Shell) Write(stream *os.File, str string) {
	if stream == os.Stderr || stream == os.Stdout {
		s.term.Write([]byte(str))
	} else {
		fmt.Fprintf(stream, str)
	}
}

func (s *Shell) ExecutePipeline(commands []*Command) {
	pipes := make([]*os.File, 0, (len(commands)-1)*2)
	defer func() {
		for _, pipe := range pipes {
			pipe.Close()
		}
	}()

	for i := 0; i < len(commands)-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating pipe: %v\n", err)
			return
		}
		pipes = append(pipes, r, w)
	}

	// processes := make([]*exec.Cmd, len(commands))
	// for i, command := range commands {
	// 	var stdin, stdout *os.File = os.Stdin, os.Stdout

	// 	if i > 0 {
	// 		stdin = pipes[(i-1)*2]
	// 	}
	// 	if i < len(commands)-1 {
	// 		stdout = pipes[i*2+1]
	// 	}
	// 	if builtin, exists := s.builtins[command.Name]; exists {
	// 		processes[i] = s.createBuiltinSubshell(command, builtin, stdin, stdout, os.Stderr)
	// 	} else {
	// 		processes[i] = s.createExternalSubshell(command, stdin, stdout, os.Stderr)
	// 	}
	// }
}

type IOFiles struct {
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

func (s *Shell) executeCommand(command *Command, f IOFiles) int {
	if builtin, exists := s.builtins[command.Name]; exists {
		exitCode := builtin(command.Args, f)
		return exitCode
	}

	cmd := exec.Command(command.Name, command.Args...)
	cmd.Stdin = f.Stdin
	cmd.Stdout = f.Stdout
	cmd.Stderr = f.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		if _, ok := err.(*exec.Error); ok {
			s.Write(f.Stderr, fmt.Sprintf("%s: executable not found"))
			return 127
		}
		fmt.Fprintf(os.Stderr, "Error executing '%s': %v\n", command.Name, err)
		return 1
	}
	return 0
}
func (s *Shell) executeExternalCommand(command *Command, f IOFiles) int {
	cmd := exec.Command(command.Name, command.Args...)
	cmd.Stdin = f.Stdin
	cmd.Stdout = f.Stdout
	cmd.Stderr = f.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "Error executing '%s': %v\n", command.Name, err)
		return 1
	}
	return 0
}

func (s *Shell) ExitCmd(args []string, f IOFiles) int {
	if len(args) > 1 {
		s.Write(f.Stderr, fmt.Sprintf("exit: %w\n", ErrTooManyArguments))
		return 128
	}

	code := 0
	if len(args) == 0 {
		var err error
		code, err = strconv.Atoi(args[0])
		if err != nil {
			s.Write(f.Stderr, fmt.Sprintf("exit: Illegal number: %s\n", args[0]))
			return 128
		}
	}
	os.Exit(code)
	return 0
}

func (s *Shell) EchoCmd(args []string, f IOFiles) int {
	str := strings.Join(args, " ") + "\n"
	s.Write(f.Stdout, str)
	return 0
}

func (s *Shell) TypeCmd(args []string, f IOFiles) int {
	if len(args) == 0 {
		return 0
	}

	for _, arg := range args {
		if _, ok := s.builtins[arg]; ok {
			s.Write(f.Stdout, fmt.Sprintf("%s is a shell builtin\n", arg))
			continue
		}
		if file, ok := findInPath(arg); ok {
			s.Write(f.Stdout, fmt.Sprintf("%s is %s\n", arg, file))
			continue
		} else {
			s.Write(f.Stderr, fmt.Sprintf("type: %s: not found\n", arg)) // inconsistent error
		}
	}
	return 0
}

func (s *Shell) PwdCmd(args []string, f IOFiles) int {
	pwd, err := os.Getwd()
	if err != nil {
		s.Write(f.Stderr, fmt.Sprintf("pwd: %w\n", err))
		return 1
	}
	s.Write(f.Stdout, pwd+"\n")
	return 0
}

func (s *Shell) CdCmd(args []string, f IOFiles) int {
	var dir string
	if len(args) == 0 {
		dir = os.Getenv("HOME")
	} else {
		dir = args[0]
	}

	if len(args) > 1 {
		s.Write(f.Stderr, fmt.Sprintf("cd: %w\n", ErrTooManyArguments))
		return 1
	}

	if dir[0] == '~' {
		HOME := os.Getenv("HOME")
		dir = strings.Replace(dir, "~", HOME, 1)
	}
	if err := os.Chdir(dir); err != nil {
		s.Write(f.Stderr, fmt.Sprintf("cd: %s: No such file or directory", dir))
		return 127
	}
	return 0
}

func (s *Shell) envCmd(args []string, f IOFiles) int {
	env := os.Environ()
	for _, e := range env {
		s.Write(f.Stdout, e)
	}
	return 0
}

func (s *Shell) HistoryCmd(args []string, f IOFiles) int {
	if len(args) > 1 {
		s.Write(f.Stderr, fmt.Sprintf("history: %w\n", ErrTooManyArguments))
		return 128
	}

	var offset = 0
	if len(args) == 0 {
		num, err := strconv.Atoi(args[0])
		offset = s.term.History.Len() - num
		if err != nil {
			s.Write(f.Stderr, fmt.Sprintf("history: Illegal number: %s", args[0]))
			return 2
		}
	}

	for i := offset; i < s.term.History.Len(); i++ {
		s.Write(f.Stdout, fmt.Sprintf("%d  %s", i+1, s.term.History.At(i)))
	}
	return 0
}

func handleRedirections(redirections []Redirection) (*IOFiles, error) {
	var files IOFiles
	for _, redir := range redirections {
		switch redir.Operator {
		case ">", ">>":
			sourceFD := 1
			if redir.Source != "" {
				var err error
				sourceFD, _ = strconv.Atoi(redir.Source)
				if err != nil {
					return nil, fmt.Errorf("redirection error: invalid source FD: %s", redir.Source)
				}
			}

			flags := os.O_WRONLY | os.O_CREATE
			if redir.Operator == ">>" {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}
			file, err := os.OpenFile(redir.Destination, flags, 0644)
			if err != nil {
				return nil, fmt.Errorf("redirection error: cannot open file %s: %v", redir.Destination, err)
			}

			rh.openFiles = append(rh.openFiles, file)

			assignFileFromFD(cmd, fd, file, &stdin, &stdout, &stderr)

		case "<":
			// Input redirection: [n]<file
			fd, file, err := rh.handleInputRedirection(redir)
			if err != nil {
				return fmt.Errorf("input redirection error: %v", err)
			}
			rh.assignFileToCommand(cmd, fd, file, &stdin, &stdout, &stderr)

		case ">&":
			// Duplicate output file descriptor: [n]>&m
			fd, file, err := rh.handleFDDuplication(redir, true)
			if err != nil {
				return fmt.Errorf("output FD duplication error: %v", err)
			}
			rh.assignFileToCommand(cmd, fd, file, &stdin, &stdout, &stderr)

		case "<&":
			// Duplicate input file descriptor: [n]<&m
			fd, file, err := rh.handleFDDuplication(redir, false)
			if err != nil {
				return fmt.Errorf("input FD duplication error: %v", err)
			}
			rh.assignFileToCommand(cmd, fd, file, &stdin, &stdout, &stderr)

		default:
			return fmt.Errorf("unsupported redirection operator: %s", redir.Operator)
		}
	}

	// Assign final file descriptors to command
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return &files, nil
}

var ErrUnexpectedEnd = errors.New("unexpected end of input")
var ErrBadFileDescriptor = errors.New("bad file descriptor")
var ErrTooManyArguments = errors.New("too many arguments")
var ErrNoRedirectionFile = errors.New("no filename or fd")
