package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

type builtin func(...string) (string, error)

var builtins = map[string]builtin{
	"exit": func(...string) (string, error) {
		os.Exit(0)
		return "", nil
	},
	"echo": func(args ...string) (string, error) {
		return strings.Join(args, " ") + "\n", nil
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
		if !ok {
			fmt.Printf("%s: command not found\n", line)
		} else {
			stdout, err := f(args...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "execute %v error: %v", cmd, err)
				continue
			}
			fmt.Fprintf(os.Stdout, stdout)
		}
	}
}
