package main

import (
	"errors"
	"fmt"
	"io"
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

type builtinCmd func(args []string, io CommandIO) int

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

func (s *Shell) Write(stream io.Writer, str string) {
	if stream == os.Stderr || stream == os.Stdout {
		s.term.Write([]byte(str))
	} else {
		fmt.Fprintf(stream, str)
	}
}

type CommandIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Command struct {
	Name         string
	Args         []string
	io           CommandIO
	isBackground bool
	Pipe         string
}

func NewCommand() *Command {
	return &Command{
		io: CommandIO{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		},
		isBackground: false,
	}
}

type Redirection struct {
	Source      any
	Operator    string
	Destination any
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

func (s *Shell) executeCommand(command *Command) int {
	if builtin, exists := s.builtins[command.Name]; exists {
		exitCode := builtin(command.Args, command.io)
		return exitCode
	}

	cmd := exec.Command(command.Name, command.Args...)
	cmd.Stdin = command.io.Stdin
	cmd.Stdout = command.io.Stdout
	cmd.Stderr = command.io.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		if _, ok := err.(*exec.Error); ok {
			s.Write(command.io.Stderr, fmt.Sprintf("%s: executable not found"))
			return 127
		}
		fmt.Fprintf(os.Stderr, "Error executing '%s': %v\n", command.Name, err)
		return 1
	}
	return 0
}

func (s *Shell) ExitCmd(args []string, io CommandIO) int {
	if len(args) > 1 {
		s.Write(io.Stderr, fmt.Sprintf("exit: %w\n", ErrTooManyArguments))
		return 128
	}

	code := 0
	if len(args) == 0 {
		var err error
		code, err = strconv.Atoi(args[0])
		if err != nil {
			s.Write(io.Stderr, fmt.Sprintf("exit: Illegal number: %s\n", args[0]))
			return 128
		}
	}
	os.Exit(code)
	return 0
}

func (s *Shell) EchoCmd(args []string, io CommandIO) int {
	str := strings.Join(args, " ") + "\n"
	s.Write(io.Stdout, str)
	return 0
}

func (s *Shell) TypeCmd(args []string, io CommandIO) int {
	if len(args) == 0 {
		return 0
	}

	for _, arg := range args {
		if _, ok := s.builtins[arg]; ok {
			s.Write(io.Stdout, fmt.Sprintf("%s is a shell builtin\n", arg))
			continue
		}
		if file, ok := findInPath(arg); ok {
			s.Write(io.Stdout, fmt.Sprintf("%s is %s\n", arg, file))
			continue
		} else {
			s.Write(io.Stderr, fmt.Sprintf("type: %s: not found\n", arg)) // inconsistent error
		}
	}
	return 0
}

func (s *Shell) PwdCmd(args []string, io CommandIO) int {
	pwd, err := os.Getwd()
	if err != nil {
		s.Write(io.Stderr, fmt.Sprintf("pwd: %w\n", err))
		return 1
	}
	s.Write(io.Stdout, pwd+"\n")
	return 0
}

func (s *Shell) CdCmd(args []string, io CommandIO) int {
	var dir string
	if len(args) == 0 {
		dir = os.Getenv("HOME")
	} else {
		dir = args[0]
	}

	if len(args) > 1 {
		s.Write(io.Stderr, fmt.Sprintf("cd: %w\n", ErrTooManyArguments))
		return 1
	}

	if dir[0] == '~' {
		HOME := os.Getenv("HOME")
		dir = strings.Replace(dir, "~", HOME, 1)
	}
	if err := os.Chdir(dir); err != nil {
		s.Write(io.Stderr, fmt.Sprintf("cd: %s: No such file or directory", dir))
		return 127
	}
	return 0
}

func (s *Shell) envCmd(args []string, io CommandIO) int {
	env := os.Environ()
	for _, e := range env {
		s.Write(io.Stdout, e)
	}
	return 0
}

func (s *Shell) HistoryCmd(args []string, io CommandIO) int {
	if len(args) > 1 {
		s.Write(io.Stderr, fmt.Sprintf("history: %w\n", ErrTooManyArguments))
		return 128
	}

	var offset = 0
	if len(args) == 0 {
		num, err := strconv.Atoi(args[0])
		offset = s.term.History.Len() - num
		if err != nil {
			s.Write(io.Stderr, fmt.Sprintf("history: Illegal number: %s", args[0]))
			return 2
		}
	}

	for i := offset; i < s.term.History.Len(); i++ {
		s.Write(io.Stdout, fmt.Sprintf("%d  %s", i+1, s.term.History.At(i)))
	}
	return 0
}

func assignFileToCommand(cmd *exec.Cmd, fd int, file *os.File, stdin, stdout, stderr **os.File) {
	if file == nil {
		// Handle FD closing (>&- or <&-)
		return
	}

	switch fd {
	case 0:
		*stdin = file
	case 1:
		*stdout = file
	case 2:
		*stderr = file
	default:
		// For FDs > 2, we'd need to use ExtraFiles
		// This is a simplified implementation
		fmt.Fprintf(os.Stderr, "Warning: FD %d redirection not fully supported\n", fd)
	}
}

func getFileFromFD(fd int) *os.File {
	switch fd {
	case 0:
		return os.Stdin
	case 1:
		return os.Stdout
	case 2:
		return os.Stderr
	default:
		fileName := fmt.Sprintf("/dev/fd/%d", fd)
		return os.NewFile(uintptr(fd), fileName)
	}
}

func handleRedirections(cmdWR CommandWRedirections) (*Command, error) {
	var command = cmdWR.Command
	for _, redir := range cmdWR.Redirections {
		switch redir.Operator {
		case ">", ">>":
			var targetStream *io.Writer
			
			if fd, ok := redir.Source.(int); ok {
				switch fd {
				case 1:
					targetStream = &command.io.Stdout
				case 2:
					targetStream = &command.io.Stderr
				default:
					return nil, fmt.Errorf("redirection error: unsupported source FD: %d", fd)
				}
			} else {
				return nil, fmt.Errorf("redirection error: invalid source FD: %s", redir.Source)
			}

			var destFile *os.File
			if fd, ok := redir.Destination.(int); ok {
				destFile = getFileFromFD(fd)
			} else {
				flags := os.O_WRONLY | os.O_CREATE
				if redir.Operator == ">>" {
					flags |= os.O_APPEND
				} else {
					flags |= os.O_TRUNC
				}
				var err error
				destFile, err = os.OpenFile(redir.Destination.(string), flags, 0644)
				if err != nil {
					return nil, fmt.Errorf("redirection error: cannot open file %s: %v", redir.Destination, err)
				}
			}
			*targetStream = destFile

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
