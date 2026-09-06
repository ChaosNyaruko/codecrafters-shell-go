package main

import (
	"bufio"
	"fmt"
	"os"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

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
		fmt.Printf("%s: command not found\n", line)
	}
}
