package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/subcommands"

	"github.com/yuin/goldmark"
	mdast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/charmbracelet/glamour"
)

var BuildID string = "unknown"

func baseBlockLines(source []byte, v *mdast.BaseBlock) string {
	s := ""
	for i := 0; i < v.Lines().Len(); i++ {
		line := v.Lines().At(i)
		s += string(line.Value(source))
	}

	return s
}

func parseInfo(info string) (string, map[string]string) {
	split := strings.Split(info, " ")
	language := split[0]

	fields := map[string]string{}
	for _, field := range split[1:] {
		splitField := strings.SplitN(field, "=", 2)
		var value string
		if len(splitField) > 1 {
			value = splitField[1]
		}
		fields[splitField[0]] = value
	}

	return language, fields
}

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

func ParseCommandDefinitions(source []byte) ([]CommandDefinition, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	node := md.Parser().Parse(text.NewReader(source))

	var heading *mdast.Heading
	var name string
	reset := func() {
		heading = nil
		name = ""
	}
	var definitions []CommandDefinition
	err := mdast.Walk(node, func(n mdast.Node, entering bool) (mdast.WalkStatus, error) {
		// fmt.Printf("n: %#v entering=%#v\n", n, entering)
		if !entering {
			return mdast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *mdast.ThematicBreak:
			reset()
		case *mdast.Heading:
			reset()
			child := v.FirstChild()
			if child == v.LastChild() {
				switch c := child.(type) {
				case *mdast.CodeSpan:
					heading = v
					name = string(c.Text(source))
				}
			}
		case *mdast.FencedCodeBlock:
			if heading != nil {
				headingLines := heading.Lines()
				headingStart := headingLines.At(0).Start
				for headingStart >= 0 {
					if headingStart == 0 || source[headingStart] == '\n' {
						break
					}
					headingStart--
				}
				headingStop := headingLines.At(headingLines.Len() - 1).Stop

				helpStart := headingStop + 1
				helpStop := v.Lines().At(0).Start
				if v.Info != nil {
					helpStop = v.Info.Segment.Start
				}
				for helpStop > helpStart {
					helpStop--
					if source[helpStop] == '\n' {
						break
					}
				}
				declarationStop := v.Lines().At(v.Lines().Len() - 1).Stop
				for declarationStop < len(source) {
					if source[declarationStop] == '\n' {
						break
					}
					declarationStop++
				}
				definitions = append(definitions, CommandDefinition{
					Source:           source,
					Name:             name,
					HeadingStart:     headingStart,
					HeadingStop:      headingStop,
					HelpStart:        helpStart,
					HelpStop:         helpStop,
					DeclarationStart: helpStop,
					DeclarationStop:  declarationStop,
					Declaration:      v,
				})
			}
			reset()
		}
		return mdast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func ParseCommands(source []byte) ([]*Command, error) {
	defs, err := ParseCommandDefinitions(source)

	if err != nil {
		return nil, err
	}

	cmds := []*Command{}
	for _, d := range defs {
		cmd, err := d.Parse()
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

type CommandDefinition struct {
	InputPath            string
	DeclaretionLineStart int

	Source           []byte
	Name             string
	HeadingStart     int
	HeadingStop      int
	HelpStart        int
	HelpStop         int
	DeclarationStart int
	DeclarationStop  int
	Declaration      *mdast.FencedCodeBlock
}

func (d *CommandDefinition) ParseHelp() string {
	return strings.TrimSpace(string(d.Source[d.HelpStart:d.HelpStop]))
}

func (d *CommandDefinition) ParseDefinition() string {
	return string(d.Source[d.HeadingStart:d.DeclarationStop])
}

func (d *CommandDefinition) ParseInfo() (string, map[string]string) {
	if d.Declaration.Info == nil {
		return "", map[string]string{}
	}

	return parseInfo(string(d.Declaration.Info.Text(d.Source)))
}

func (d *CommandDefinition) ParseCommand() string {
	return baseBlockLines(d.Source, &d.Declaration.BaseBlock)
}

func appendStrings(s []string, o []interface{}) []string {
	for _, arg := range o {
		s = append(s, fmt.Sprintf("%s", arg))
	}
	return s
}

const DefaultLanguage = "bash"

func (d *CommandDefinition) Parse() (*Command, error) {
	language, fields := d.ParseInfo()
	// alias := fields["alias"]

	if language == "" {
		language = DefaultLanguage
	}

	text := d.ParseCommand()

	var execCommand = language
	var checkCommand = language

	setExecFlags := func(f *flag.FlagSet) {}
	var renderCheckCmd func(ctx context.Context) *exec.Cmd
	var renderExecCmd func(ctx context.Context, f *flag.FlagSet, args ...interface{}) (*exec.Cmd, error)

	switch language {
	case "bash", "sh", "shell":
		if language == "shell" {
			execCommand = os.Getenv("SHELL")
			checkCommand = execCommand
		}

		shellCommand, err := NewShellCommand(text)
		if err != nil {
			return nil, err
		}

		setExecFlags = func(f *flag.FlagSet) {
			for l, defaultValue := range shellCommand.Locals {
				var v string
				if defaultValue == nil {
					f.StringVar(&v, l, os.Getenv(l), fmt.Sprintf("falls back to $%s", l))
				} else {
					if defaultValue.Literal != "" {
						f.StringVar(&v, l, defaultValue.Literal, fmt.Sprintf("falls back to $%s", l))
					} else {
						f.StringVar(&v, l, "", fmt.Sprintf("falls back to $%s (default is expression %s)", l, defaultValue.Expression))
					}
				}
			}
		}

		// From the bash manual page:
		// If the -c option is present, then commands are read from string.  If there are arguments after the string, they are assigned to the positional parameters, starting with $0.
		renderExecCmd = func(ctx context.Context, f *flag.FlagSet, args ...interface{}) (*exec.Cmd, error) {
			for env, defaultValue := range shellCommand.Exports {
				if defaultValue == nil && os.Getenv(env) == "" {
					return nil, fmt.Errorf("environment variable not set: %s", env)
				}
			}

			locals := map[string]*ShellValue{}

			for l, defaultValue := range shellCommand.Locals {
				var v string
				flag := f.Lookup(l)
				set := false

				if flag != nil {
					v = flag.Value.String()
					set = v != ""
				}

				if !set && defaultValue == nil {
					return nil, fmt.Errorf("option not given: %s", l)
				}

				if set {
					locals[l] = &ShellValue{Literal: v}
				}
			}

			rendered, err := shellCommand.Render(locals)
			if err != nil {
				return nil, err
			}
			return exec.CommandContext(ctx,
				execCommand,
				append(
					[]string{"-c", rendered, fmt.Sprint(args[0])},
					f.Args()...,
				)...,
			), nil
		}
		renderCheckCmd = func(ctx context.Context) *exec.Cmd {
			return exec.CommandContext(ctx, checkCommand, "-n", "-c", text)
		}
	case "python":
		// From the python manual page:
		//   -c command
		//          Specify the command to execute (see next section).  This terminates the option list (following options are passed as arguments to the command).
		renderExecCmd = func(ctx context.Context, f *flag.FlagSet, args ...interface{}) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, language, appendStrings([]string{"-c", text}, args)...), nil
		}
		renderCheckCmd = func(ctx context.Context) *exec.Cmd {
			return exec.CommandContext(ctx, checkCommand, "-c", "import ast, sys; ast.parse(sys.argv[1])", text)
		}
	case "node":
		renderExecCmd = func(ctx context.Context, f *flag.FlagSet, args ...interface{}) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, language, appendStrings([]string{"--eval", text, "--"}, args)...), nil
		}
		renderCheckCmd = func(ctx context.Context) *exec.Cmd {
			return exec.CommandContext(ctx, checkCommand, "--eval", `const vm = require('vm'); new vm.Script(process.argv[1])`, "--", text)
		}
	default:
		// TODO Or maybe as a fallback we write the command to a tempfile just before we need it?
		// execArgs = []string{"-c", d.ParseCommand()}
		return nil, fmt.Errorf("unknown language for code block: %s", language)
	}

	dockerImage := fields["image"]
	if dockerImage != "" {
		dockerPath, err := exec.LookPath("docker")
		if err != nil {
			return nil, err
		}

		innerRenderExecCmd := renderExecCmd
		renderExecCmd = func(ctx context.Context, f *flag.FlagSet, args ...interface{}) (*exec.Cmd, error) {
			cmd, err := innerRenderExecCmd(ctx, f, args...)
			if err != nil {
				return nil, err
			}
			cmd.Args = append([]string{"run", dockerImage, cmd.Path}, cmd.Args...)
			cmd.Path = dockerPath
			return cmd, nil
		}

		innerRenderCheckCmd := renderCheckCmd
		renderCheckCmd = func(ctx context.Context) *exec.Cmd {
			cmd := innerRenderCheckCmd(ctx)
			cmd.Args = append([]string{"run", dockerImage, cmd.Path}, cmd.Args...)
			cmd.Path = dockerPath
			return cmd
		}
	}

	return &Command{
		Help:       d.ParseHelp(),
		Definition: d.ParseDefinition(),

		Alias: d.Name,
		Group: fields["group"],

		RenderCheckCmd: renderCheckCmd,

		SetExecFlags:  setExecFlags,
		RenderExecCmd: renderExecCmd,
	}, nil
}

type Command struct {
	Alias string
	Group string

	Help       string
	Definition string

	RenderCheckCmd func(ctx context.Context) *exec.Cmd

	SetExecFlags  func(f *flag.FlagSet)
	RenderExecCmd func(ctx context.Context, f *flag.FlagSet, args ...interface{}) (*exec.Cmd, error)
}

func (c *Command) Name() string { return c.Alias }
func (c *Command) Synopsis() string {
	split := strings.SplitN(c.Help, ".", 2)
	split = strings.SplitN(split[0], "\n\n", 2)
	return split[0]
}
func (c *Command) Usage() string {
	// return fmt.Sprintf("%s\n%s\n", c.Alias, c.Help)
	out, err := glamour.Render(c.Definition, "dark")
	if err != nil {
		return c.Definition
	}
	return out
}
func (c *Command) SetFlags(f *flag.FlagSet) {
	c.SetExecFlags(f)
}
func (c *Command) Check(ctx context.Context) (string, error) {
	cmd := c.RenderCheckCmd(ctx)
	// fmt.Fprintf(os.Stderr, ">>> executing command path=%s args=%#v\n", cmd.Path, cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

func (c *Command) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	cmd, err := c.RenderExecCmd(ctx, f, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		return subcommands.ExitUsageError
	}
	// fmt.Fprintf(os.Stderr, ">>> executing command path=%s args=%#v\n", cmd.Path, cmd.Args)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "err: %s", err)
		return subcommands.ExitFailure
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return subcommands.ExitStatus(exitErr.ExitCode())
		}

		fmt.Fprintf(os.Stderr, "err: %s", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}
