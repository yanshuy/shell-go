package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/term"
)

type Shell struct {
	term         *term.Terminal
	jobs         map[int]*Pipeline
	jobCounter   int
	mu           sync.RWMutex
	workingDir   string
	env          map[string]string
	builtins     map[string]BuiltinCmd
	sigChan      chan os.Signal
	lastExitCode int
}

type BuiltinCmd func(args []string, stdin io.Reader, stdout, stderr io.Writer) int

func NewShell(doneChan <-chan bool) (*Shell, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		return nil, errors.New("stdin is not a terminal")
	}

	t := term.NewTerminal(os.Stdin, "$ ")

	prevState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("error setting raw mode: %w", err)
	}

	go func() {
		<-doneChan
		term.Restore(int(os.Stdin.Fd()), prevState)
	}()

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	shell := &Shell{
		term:         t,
		jobs:         make(map[int]*Job),
		jobCounter:   1,
		env:          make(map[string]string),
		workingDir:   wd,
		lastExitCode: 0,
		sigChan:      make(chan os.Signal, 1),
	}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			shell.env[parts[0]] = parts[1]
		}
	}
	shell.env["SHELL"] = "goson"

	signal.Notify(shell.sigChan, syscall.SIGINT, syscall.SIGTSTP, syscall.SIGCHLD)
	go func() {
		for sig := range shell.sigChan {
			switch sig {
			case syscall.SIGINT:
				shell.handleSIGINT()
			case syscall.SIGTSTP:
				shell.handleSIGTSTP()
			case syscall.SIGCHLD:
				shell.handleSIGCHLD()
			case syscall.SIGTERM:
				shell.handleSIGCHLD()
			}
		}
	}()

	shell.builtins = map[string]BuiltinFunc{
		"exit":    s.Exit,
		"echo":    s.Echo,
		"type":    s.Type,
		"pwd":     s.Pwd,
		"cd":      s.Cd,
		"env":     s.Env,
		"history": s.History,
		"export":  s.Export,
		"unset":   s.Unset,
		"jobs":    s.Jobs,
		"fg":      s.Fg,
		"bg":      s.Bg,
		"kill":    s.Kill,
	}
	return shell, nil
}

func (s *Shell) handleSIGINT() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, job := range s.jobs {
		if !job.Background && job.Status == JobRunning {
			job.ProcessGroup.Signal(syscall.SIGINT)
			break
		}
	}
}

func (s *Shell) handleSIGTSTP() {
	// Stop current foreground job
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, job := range s.jobs {
		if !job.Background && job.Status == JobRunning {
			job.ProcessGroup.Signal(syscall.SIGTSTP)
			job.Status = JobStopped
			fmt.Printf("\n[%d]+  Stopped    %s\n", job.ID, job.String())
			break
		}
	}
}

func (s *Shell) handleSIGCHLD() {
	// Clean up completed background jobs
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, job := range s.jobs {
		if job.Background && job.Pipeline.IsCompleted() {
			exitCode, _ := job.Pipeline.Wait()
			job.Status = JobCompleted
			fmt.Printf("[%d]   Done (%d)   %s\n", job.ID, exitCode, job.String())
			delete(s.jobs, id)
		}
	}
}

func (s *Shell) Write(stream io.Writer, str string) {
	if stream == os.Stderr || stream == os.Stdout {
		s.term.Write([]byte(str))
	} else {
		fmt.Fprintf(stream, str)
	}
}

type Command struct {
	Name     string
	Args     []string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	extraFDs map[int]*os.File

	Process  *os.Process
	exitCode int
	done     chan struct{}
}

func NewCommand(name string, args []string) *Command {
	return &Command{
		Name:     name,
		Args:     args,
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		extraFDs: make(map[int]*io.ReadWriter),
		done:     make(chan struct{}),
		exitCode: -1,
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

func (s *Shell) executeCommand(command *Command) int {
	if builtin, exists := s.builtins[command.Name]; exists {
		exitCode := builtin(command.Args, command.done)
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

func handleRedirections(cmdWR CommandWRedirections) (*Command, error) {
	var pcommand = cmdWR.Command
	for _, redir := range cmdWR.Redirections {
		switch redir.Operator {
		case ">", ">>":
			var targetStream *io.Writer

			if fd, ok := redir.Source.(int); ok {
				switch fd {
				case 1:
					targetStream = &pcommand.io.Stdout
				case 2:
					targetStream = &pcommand.io.Stderr
				default:
					file := getFileFromFD(fd)
					if file != nil {
						var a io.Writer = file
						targetStream = &a
					} else {
						return nil, fmt.Errorf("redirection error: cannot get file for FD: %d", fd)
					}
				}
			} else {
				return nil, fmt.Errorf("redirection error: invalid source FD: %v", redir.Source)
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
			var targetStream *io.Reader

			if fd, ok := redir.Destination.(int); ok {
				switch fd {
				case 0:
					targetStream = &pcommand.io.Stdin
				default:
					return nil, fmt.Errorf("redirection error: unsupported target FD: %d", fd)
				}
			} else {
				return nil, fmt.Errorf("redirection error: invalid target FD: %s", redir.Source)
			}

			var sourceFile *os.File
			if fd, ok := redir.Source.(int); ok {
				sourceFile = getFileFromFD(fd)
			} else {
				var err error
				sourceFile, err = os.OpenFile(redir.Source.(string), os.O_RDONLY, 0644)
				if err != nil {
					return nil, fmt.Errorf("redirection error: cannot open file %s: %v", redir.Source, err)
				}
			}
			*targetStream = sourceFile

		default:
			return nil, fmt.Errorf("unsupported redirection operator: %s", redir.Operator)
		}
	}

	return pcommand, nil
}

var ErrUnexpectedEnd = errors.New("unexpected end of input")
var ErrBadFileDescriptor = errors.New("bad file descriptor")
var ErrTooManyArguments = errors.New("too many arguments")
var ErrNoRedirectionFile = errors.New("no filename or fd")
