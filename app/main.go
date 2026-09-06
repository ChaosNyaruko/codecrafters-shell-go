package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

type builtin func(...string) (string, error)

var builtinSet = map[string]struct{}{
	"exit": {},
	"echo": {},
	"type": {},
}

var builtins = map[string]builtin{
	"exit": func(...string) (string, error) {
		os.Exit(0)
		return "", nil
	},
	"echo": func(args ...string) (string, error) {
		return strings.Join(args, " ") + "\n", nil
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

func parseInput(line string) (string, []string, error) {
	parts := strings.Split(line, " ")
	cmd := parts[0]
	args := make([]string, len(parts[1:]))
	copy(args, parts[1:])
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
		f, ok := builtins[cmd]
		if ok {
			stdout, err := f(args...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "execute %v error: %v", cmd, err)
			} else {
				fmt.Fprintf(os.Stdout, stdout)
			}
			continue
		}
		proc := exec.Command(cmd, args...)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		proc.Stdin = os.Stdin
		err = proc.Run()
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Printf("%s: command not found\n", cmd)
			} else {
				fmt.Fprintf(os.Stderr, "exec %s error: %v\n", cmd, err)
			}
		}
	}
}
