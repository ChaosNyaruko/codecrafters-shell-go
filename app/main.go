package main

import (
	"bufio"
	"fmt"
	"os"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

type builtin func(...string) string

var builtins = map[string]builtin{
	"exit": func(...string) string {
		os.Exit(0)
		return ""
	}}

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
		f, ok := builtins[line]
		if !ok {
			fmt.Printf("%s: command not found\n", line)
		} else {
			f("")
		}
	}
}
