package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

type builtin func(...string) (string, error)

var builtinSet = map[string]struct{}{
	"exit": {},
	"echo": {},
	"type": {},
	"pwd":  {},
	"cd":   {},
}

func init() {
}

var builtins = map[string]builtin{
	"cd": func(args ...string) (string, error) {
		if len(args) == 0 {
			return "\n", nil
		}
		path := args[0]
		var err error
		var aPath string
		if strings.TrimSpace(path) == "~" {
			aPath = os.Getenv("HOME")
		} else {
			aPath, err = filepath.Abs(path)
		}
		if err != nil {
			return "", err
		}
		err = os.Chdir(aPath)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Sprintf("cd: %s: No such file or directory\n", aPath), nil
		}
		return "", err
	},
	"pwd": func(...string) (string, error) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd + "\n", nil
	},
	"exit": func(...string) (string, error) {
		os.Exit(0)
		return "", nil
	},
	"echo": func(args ...string) (string, error) {
		lf := "\n"
		echoFrom := 0
		if len(args) >= 1 && args[0] == "-n" {
			lf = ""
			echoFrom += 1
		}
		return fmt.Sprintf("%s%s", strings.Join(args[echoFrom:], " "), lf), nil
	},
	"type": func(args ...string) (string, error) {
		if len(args) == 0 {
			return "\n", nil
		}
		cmd := args[0]
		_, ok := builtinSet[cmd]
		if ok {
			return fmt.Sprintf("%s is a shell builtin\n", cmd), nil
		}
		path, err := exec.LookPath(cmd)
		if err == nil {
			return fmt.Sprintf("%s is %s\n", cmd, path), nil
		}
		return fmt.Sprintf("%s: not found\n", cmd), nil
	},
}

const (
	normal = iota
	singleQuoteStarted
	doubleQuoteStarted
	escaping
)

var normalEscapes = map[rune]rune{
	// NOTE: without quotes, we just "escape" as-is
}

var quotedEscapes = map[rune]rune{
	'n':  '\n',
	't':  '\t',
	'\'': '\'',
	'"':  '"',
	' ':  ' ',
	'$':  '?', // TODO: we don't support env vars
}

func parseInput(line string) (string, []string, error) {
	status := normal
	beStatus := normal // before escaping status
	inputs := make([]string, 0, 2)
	var cur string
	for _, c := range line {
		switch status {
		case singleQuoteStarted:
			beStatus = singleQuoteStarted
			if c == '\'' {
				status = normal
			} else {
				cur += string(c)
			}
		case doubleQuoteStarted:
			// echo "example  hello"  "test""world"
			beStatus = doubleQuoteStarted
			if c == '"' {
				status = normal
			} else if c == '\\' {
				status = escaping
			} else {
				cur += string(c)
			}
		case normal:
			beStatus = normal
			if c == '"' {
				status = doubleQuoteStarted
			} else if c == '\'' {
				status = singleQuoteStarted
			} else if c == ' ' {
				if cur != "" {
					inputs = append(inputs, cur)
					cur = ""
				} else {
					continue
				}
			} else if c == '\\' {
				status = escaping
			} else {
				cur += string(c)
			}
		case escaping:
			es := normalEscapes
			if beStatus == doubleQuoteStarted {
				es = quotedEscapes
			}
			escaped, ok := es[c]
			// TODO: not processing "$" as envs yet
			if !ok {
				cur += string(c)
			} else {
				cur += string(escaped)
			}
			status = beStatus
		default:
		}

	}

	if status != normal {
		return "", nil, fmt.Errorf("bad status: %v", status)
	}

	if cur != "" {
		inputs = append(inputs, cur)
	}
	cmd := inputs[0]
	args := inputs[1:]
	// fmt.Fprintf(os.Stderr, "parse inputs: cmd: %s, args: %#v\n", cmd, args)
	return cmd, args, nil
}

func main() {
	// REPL:
	// read/eval/print/loop
	for {
		fmt.Print("$ ")
		scanner := bufio.NewScanner(os.Stdin)
		scanned := scanner.Scan()
		if !scanned {
			os.Exit(69)
		}
		line := scanner.Text()
		cmd, args, err := parseInput(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v", err)
			continue
		}
		args, stdout, stderr, err := getRedirectIfPossible(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "redirect error : %v", err)
			continue
		}
		f, ok := builtins[cmd]
		if ok {
			output, err := f(args...)
			if err != nil {
				// fmt.Fprintf(stderr, "execute %v error: %v", cmd, err)
			} else {
				fmt.Fprintf(stdout, "%s", output)
			}
			continue
		}
		proc := exec.Command(cmd, args...)
		proc.Stdout = stdout
		proc.Stderr = stderr
		proc.Stdin = os.Stdin
		err = proc.Run()
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Printf("%s: command not found\n", cmd)
			} else {
				// fmt.Fprintf(os.Stderr, "exec %s error: %v\n", cmd, err)
			}
		}
	}
}

// getRedirectIfPossible returns args after trimming redirect directives, redirected stdout, redirected, stderr
func getRedirectIfPossible(args []string) ([]string, *os.File, *os.File, error) {
	if len(args) < 2 {
		return args, os.Stdout, os.Stderr, nil
	}
	n := len(args)
	if args[n-2] == ">" || args[n-2] == "1>" {
		fname := args[n-1]
		outfd, err := os.Create(fname)
		if err != nil {
			return args, os.Stdout, os.Stderr, err
		}
		return args[:n-2], outfd, os.Stderr, nil
	}
	return args, os.Stdout, os.Stderr, nil
}
