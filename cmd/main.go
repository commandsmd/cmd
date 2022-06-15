package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/commandsmd/cmd"

	getter "github.com/hashicorp/go-getter"

	"github.com/google/subcommands"
)

func UpWhere(initialDir, marker string) (string, error) {
	dir := initialDir
	for {
		_, err := os.Stat(path.Join(dir, marker))
		if err == nil {
			return dir, nil
		}

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return dir, err
		}

		parent := path.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("couldn't find %s in any directory between %s and %s", marker, dir, initialDir)
		}
		dir = parent
	}
}

func readFromURL(url string) ([]byte, error) {
	f, err := ioutil.TempFile("", "*.md")
	if err != nil {
		return nil, err
	}

	defer os.Remove(f.Name())
	defer f.Close()
	err = getter.GetFile(f.Name(), url)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(f)
}

func readInput(inputPath string) ([]byte, error) {
	if inputPath == "-" {
		return io.ReadAll(os.Stdin)
	}

	if strings.HasPrefix(inputPath, ".../") {
		marker := strings.TrimPrefix(inputPath, ".../")
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		dir, err := UpWhere(cwd, marker)
		if err != nil {
			return nil, err
		}
		inputPath = path.Join(dir, marker)
	}

	f, err := os.Open(inputPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readFromURL(inputPath)
		}
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

type checkCommand struct {
	name     string
	commands []*cmd.Command
}

func (c *checkCommand) Name() string     { return c.name }
func (c *checkCommand) Synopsis() string { return "check for syntax errors" }
func (c *checkCommand) Usage() string {
	return `check [command]
Checks the syntax for all commands or just the given one
`
}
func (c *checkCommand) SetFlags(f *flag.FlagSet) {
	// ...
}

func (c *checkCommand) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	failed := false
	var only map[string]bool
	if len(args) > 1 {
		only = map[string]bool{}
		only[args[1].(string)] = true
	}

	for _, cmd := range c.commands {
		if only != nil && !only[cmd.Alias] {
			continue
		}

		out, err := cmd.Check(ctx)
		if err != nil {
			fmt.Printf("error	%s: %s\n", cmd.Alias, err)
			failed = true
		} else {
			fmt.Printf("ok		%s\n", cmd.Alias)
		}
		for _, line := range strings.Split(out, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}

	if failed {
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func main() {
	// If both are set, we seem to be running within a bazel-run environment.
	if os.Getenv("BUILD_WORKSPACE_DIRECTORY") != "" && os.Getenv("BUILD_WORKING_DIRECTORY") != "" {
		if err := os.Chdir(os.Getenv("BUILD_WORKING_DIRECTORY")); err != nil {
			panic(err)
		}
	}

	var inputPath string
	restIndex := 1
	for _, arg := range os.Args[restIndex:] {
		restIndex++
		if arg == "--" {
			break
		}

		inputPath = arg
		break
	}
	if inputPath == "" {
		inputPath = "-"
	}

	source, err := readInput(inputPath)
	if err != nil {
		panic(err)
	}

	args := []interface{}{}
	for _, arg := range os.Args[restIndex:] {
		args = append(args, arg)
	}

	cmds, err := cmd.ParseCommands(source)
	if err != nil {
		panic(err)
	}

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(&checkCommand{
		name:     "check",
		commands: cmds,
	}, "")

	// subcommands.Register(subcommands.FlagsCommand(), "")
	// subcommands.Register(subcommands.CommandsCommand(), "")

	for _, c := range cmds {
		if c.Alias == "" {
			continue
		}
		group := c.Group
		if group == "" {
			group = "default"
		}
		subcommands.Register(c, group)
	}
	parseArgs := append([]string{}, os.Args[restIndex:]...)

	if err = flag.CommandLine.Parse(parseArgs); err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx, args...)))
}
