package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chzyer/readline"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

type builtin func(...string) (string, error)

var builtinSet = map[string]struct{}{
	"exit":     {},
	"echo":     {},
	"type":     {},
	"pwd":      {},
	"cd":       {},
	"complete": {},
}

func init() {
}

var compdb = &CompleteDB{db: make(map[string]*CompletionCommand)}

type CompleteDB struct {
	// db: cmd -> specification
	db map[string]*CompletionCommand
}

type CompletionCommand struct {
	command string
	name    string
}

func (cc *CompletionCommand) String() string {
	return fmt.Sprintf("complete -C '%s' %s", cc.command, cc.name)
}

var builtins = map[string]builtin{
	"complete": func(args ...string) (string, error) {
		if len(args) == 0 {
			return "\n", nil
		}
		switch args[0] {
		case "-p": // -p for print
			if len(args) < 2 {
				return "-p recevied a <cmd> as argument", nil
			}
			spec, ok := compdb.db[args[1]]
			if !ok {
				return fmt.Sprintf("complete: %s: no completion specification\n", args[1]), nil
			}
			return spec.String() + "\n", nil
		case "-C": // -C for custom command
			if len(args) < 3 {
				return fmt.Sprintf("-C accepts 2 arguments, but got only %d", len(args[0])), nil
			}
			command := args[1]
			name := args[2]
			compdb.db[name] = &CompletionCommand{
				command: command,
				name:    name,
			}
			return "", nil
		default:
			return fmt.Sprintf("unsupported 'complete' flag: %v\n", args[0]), nil
		}
	},
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

func parseInput(line string) (string, []string, int, error) {
	status := normal
	cmdEndAt := len(line)
	beStatus := normal // before escaping status
	inputs := make([]string, 0, 2)
	var cur string
	for i, c := range line {
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
					if len(inputs) == 0 {
						// NOTE: might have bug in non-ASCII unicodes
						cmdEndAt = i
					}
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
		return "", nil, cmdEndAt, fmt.Errorf("bad status: %v", status)
	}

	if cur != "" {
		inputs = append(inputs, cur)
	}
	cmd := inputs[0]
	args := inputs[1:]
	// fmt.Fprintf(os.Stderr, "parse inputs: cmd: %s, args: %#v\n", cmd, args)
	return cmd, args, cmdEndAt, nil
}

const (
	tabInit    = 0
	tabPending = 1 // completing
)

const (
	completionCmdMode = iota
	completionFilenameMode
	completionProgrammable
)

type CommandCompleter struct {
	tabStatus int
	instance  *readline.Instance
}

// Readline will pass the whole line and current offset to it
// Completer need to pass all the candidates, and how long they shared the same characters in line
// Example:
//
//	[go, git, git-shell, grep]
//	Do("g", 1) => ["o", "it", "it-shell", "rep"], 1
//	Do("gi", 2) => ["t", "t-shell"], 2
//	Do("git", 3) => ["", "-shell"], 3
func (cc *CommandCompleter) Do(line []rune, pos int) (newline [][]rune, length int) {
	// fmt.Printf("[do]line: %v, pos: %v, status: %v\n", line, pos, cc.tabStatus)
	candidates := []string{}
	seen := make(map[string]struct{})
	// build candidates
	// NOTE: using Trie might be a good idea to improve the perf, but we don't need it yet.
	// prefix is the prefix of "to be completed item", which is gotten by "space(shell semantics)-split", a.k.a parseInput
	mode, script, cmd, prefix, previous := getCompletionMode(line, pos)
	switch mode {
	case completionCmdMode:
		for cmd := range builtinSet {
			if strings.HasPrefix(cmd, string(prefix)) {
				candidates = append(candidates, cmd+" ")
				seen[cmd] = struct{}{}
			}
		}
		exes := getAllExecutables()
		for _, cmd := range exes {
			_, ok := seen[cmd]
			if ok {
				continue
			}
			if strings.HasPrefix(cmd, string(prefix)) {
				candidates = append(candidates, cmd+" ")
				seen[cmd] = struct{}{}
			}
		}
	case completionFilenameMode:
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		dir, file := filepath.Split(string(prefix))
		// fmt.Printf("dir: %v, file: %v\n", dir, file)
		ents, err := os.ReadDir(filepath.Join(wd, dir))
		if err != nil {
			return [][]rune{}, 0
		}
		for _, ent := range ents {
			name := ent.Name()
			if strings.HasPrefix(name, file) {
				if ent.IsDir() {
					dirName := filepath.Join(dir, name) + string(filepath.Separator)
					// fmt.Printf("dirname: %v\n", dirName)
					candidates = append(candidates, dirName)
				} else {
					candidates = append(candidates, filepath.Join(dir, name)+" ")
				}
			}
		}
	case completionProgrammable:
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		s := script
		if !filepath.IsAbs(s) {
			s, err = filepath.Abs(filepath.Join(wd, script))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "get script abs path err: %v", err)
			break
		}
		scmd := exec.Command(s, []string{cmd, string(prefix), previous}...)
		buf := bytes.NewBuffer([]byte{})
		scmd.Stdout = buf
		if e := scmd.Run(); e != nil {
			fmt.Fprintf(os.Stderr, "completion script %s run failed: %v", s, e)
			break
		}
		// fmt.Fprintf(os.Stderr, "%q\n", buf.String())
		for item := range strings.SplitSeq(buf.String(), "\n") {
			if item != "" {
				candidates = append(candidates, item+" ")
			}
		}
	default:
		panic(fmt.Sprintf("unreachable completion mode: %v", mode))
	}
	// fmt.Printf("[get mode] line: %v, pos: %d, mode: %d, prefix: %q\n", line, pos, mode, prefix)
	// fmt.Printf("[candidates]: %v\n", candidates)
	if len(candidates) == 0 {
		cc.tabStatus = tabInit
		cc.instance.Terminal.Bell()
		return [][]rune{}, 0
	}
	if len(candidates) == 1 {
		cc.tabStatus = tabInit
		return [][]rune{[]rune(candidates[0][len(prefix):])}, pos
	}

	// partial completion
	commonPrefix := longestCommonPrefix(candidates)
	// fmt.Printf("common prefix: %q, pos: %d\n", commonPrefix, pos)
	if len(commonPrefix) > len(prefix) {
		// gr|
		// gr|e
		// gr|ub-mk
		// gr|ub-me
		// ....
		return [][]rune{[]rune(commonPrefix[len(prefix):])}, len(commonPrefix)
	}

	if cc.tabStatus == tabInit {
		cc.tabStatus = tabPending
		cc.instance.Terminal.Bell()
		return [][]rune{}, 0
	}
	// fmt.Printf("pending: candidates: %v\n", candidates)
	// else consecutive tabs
	// NOTE: very bad performance, but we are focusing on the correctness now
	first := true
	slices.Sort(candidates)
	for _, cmd := range candidates {
		if first {
			cc.instance.Terminal.Write([]byte("\n" + string(cmd)))
			first = false
		} else {
			cc.instance.Terminal.Write([]byte(" " + string(cmd)))
		}
	}
	// NOTE: We need "flush" to pass the test
	cc.instance.Terminal.Write([]byte{'\n'})
	// BUG: when <Tab>ed once, all subsequent tabs will not trigger a bell when multiple cands exist, due to non-completed status management
	//   NOTE: we somehow fixed it by resetting tab status every time when the second tab triggered,
	//         it conforms to the spec that codecrafters gives, but not consistent to bash,
	//         which doesn't need double tab when the same prefix is required for completion
	cc.tabStatus = tabInit
	// OnChanged really has bizarre behaviour, if enabled (and without this flush, test failed), the output will have a leading prompt, which is not what we want
	cc.instance.Terminal.Write([]byte("$ " + string(line)))
	return [][]rune{}, 0
}

// getCompletionMode returns (mode, script, cmd/argv[1], prefix/completingargv[2]), previous/argv[3])
func getCompletionMode(line []rune, pos int) (int, string, string, []rune, string) {
	cmd, args, endAt, err := parseInput(string(line))
	if err != nil {
		// we don't know what will cause the error, so use filename mode for now
		return completionFilenameMode, "", "", []rune{}, ""
	}
	// gre xxx yyy
	if pos <= endAt {
		return completionCmdMode, "", cmd, line[:pos], ""
	}

	// TODO: we just use the last arg as prefix for now
	if script, ok := compdb.db[cmd]; ok {
		if len(args) == 0 {
			return completionProgrammable, script.command, cmd, []rune{}, ""
		}
		previous := cmd
		if len(args) > 1 {
			previous = args[len(args)-2]
		}
		return completionProgrammable, script.command, cmd, []rune(args[len(args)-1]), previous
	}

	// TODO: we just use the last arg as prefix for now
	if len(args) == 0 {
		return completionFilenameMode, "", cmd, []rune{}, ""
	}
	return completionFilenameMode, "", cmd, []rune(args[len(args)-1]), ""
}

func longestCommonPrefix(candidates []string) string {
	// len >=1 is assured
	prefix := candidates[0]
	for _, str := range candidates[1:] {
		j := 0
		for ; j < len(prefix) && j < len(str) && str[j] == prefix[j]; j++ {
		}
		prefix = prefix[:j]
		if len(prefix) == 0 {
			break
		}
	}
	return prefix
}

func getAllExecutables() []string {
	seen := make(map[string]struct{})
	res := []string{}
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			m := info.Mode()
			name := info.Name()
			// owner rwx oct
			// group rwx
			// other rwx
			//       1
			//        1
			//         1
			if m&0o111 != 0 {
				// NOTE: we don't cover basic unix ACL
				if _, ok := seen[name]; !ok {
					res = append(res, name)
					seen[name] = struct{}{}
				}
			}
		}
	}
	return res
}

//	type Listener interface {
//		OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)
//	}

func (cc *CommandCompleter) OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
	// fmt.Printf("[onchange]line: %v, pos: %d, key: %v\n", line, pos, key)
	if cc.tabStatus == tabPending && key != '\t' {
		cc.tabStatus = tabInit
		return line, pos, true
	}
	return line, pos, true
}

func main() {
	// REPL:
	// read/eval/print/loop
	cc := &CommandCompleter{}
	ins, err := readline.NewEx(&readline.Config{
		// shebang
		Prompt:       "$ ",
		AutoComplete: cc,
		Listener:     nil,
	})
	cc.instance = ins
	if err != nil {
		panic(err)
	}
	defer ins.Close()
	for {
		line, err := ins.Readline()
		if err != nil {
			fmt.Fprintf(os.Stderr, "readline error: %v", err)
			continue
		}
		cmd, args, _, err := parseInput(line)
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
				_, err := fmt.Fprintf(stdout, "%s", output)
				if err != nil {
					panic(err)
				}
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
	outfd := os.Stdout
	errfd := os.Stderr
	var err error

	if len(args) < 2 {
		return args, os.Stdout, os.Stderr, nil
	}
	i := len(args)
	argsEnd := len(args)
	for ; i-1 >= 0 && i-2 >= 0; i -= 2 {
		// right side takes higher precedence
		if (outfd == os.Stdout) && (args[i-2] == ">" || args[i-2] == "1>") {
			fname := args[i-1]
			outfd, err = os.Create(fname)
			if err != nil {
				return args, os.Stdout, os.Stderr, err
			}
			argsEnd -= 2
		} else if (outfd == os.Stdout) && (args[i-2] == ">>" || args[i-2] == "1>>") {
			fname := args[i-1]
			outfd, err = os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
			if err != nil {
				return args, os.Stdout, os.Stderr, err
			}
			argsEnd -= 2
		} else if (errfd == os.Stderr) && args[i-2] == "2>>" {
			fname := args[i-1]
			errfd, err = os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
			if err != nil {
				return args, os.Stdout, os.Stderr, err
			}
			argsEnd -= 2
		} else if (errfd == os.Stderr) && args[i-2] == "2>" {
			fname := args[i-1]
			errfd, err = os.Create(fname)
			if err != nil {
				return args, os.Stdout, os.Stderr, err
			}
			argsEnd -= 2
		}
		if outfd != os.Stdout && errfd != os.Stderr {
			break
		}
	}
	return args[:argsEnd], outfd, errfd, nil
}
