package cmd_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	cmd "github.com/commandsmd/cmd"
)

func lit(s string) *cmd.ShellValue {
	return &cmd.ShellValue{Literal: s}
}

func expr(s string) *cmd.ShellValue {
	return &cmd.ShellValue{Expression: s}
}

func TestShellCommandParsing(t *testing.T) {

	type scenario struct {
		s      string
		locals map[string]*cmd.ShellValue
	}

	var cases = []struct {
		name    string
		source  string
		command *cmd.ShellCommand

		scenarios []scenario
	}{
		{
			name: "explicit exports",
			source: `
export S
export Y=1

foo() {
	export Z
	export W=2
}
		`,
			command: &cmd.ShellCommand{
				Exports: map[string]*cmd.ShellValue{"S": nil, "Y": lit("1")},
				Locals:  map[string]*cmd.ShellValue{},
			},
		},
		{
			name: "explicit locals",
			source: `
: ${S}

: ${Y:=1}

K=3

J=$(echo)

foo() {
	: ${Z}
 	: ${W:=2}
}
`,
			command: &cmd.ShellCommand{
				Exports: map[string]*cmd.ShellValue{},
				Locals:  map[string]*cmd.ShellValue{"S": nil, "Y": lit("1"), "K": lit("3"), "J": expr("$(echo)")},
			},
			scenarios: []scenario{
				{
					s: `: ${S:=s}

: ${Y:=2}

K=3

J=$(echo)

foo() {
	: ${Z}
	: ${W:=2}
}
`,
					locals: map[string]*cmd.ShellValue{
						"S": lit("s"),
						"Y": lit("2"),
						// "J": expr("$(echo)"),
					},
				},
			},
		},
		{
			name: "discover locals",
			source: `
		echo $1 $@ $# $? ${S} ${Y:=1} ${Z:-2}
		`,
			command: &cmd.ShellCommand{
				Exports: map[string]*cmd.ShellValue{},
				Locals:  map[string]*cmd.ShellValue{"S": nil, "Y": lit("1"), "Z": lit("2")},
			},
		},
		{
			name: "discover exports",
			source: `
export Y
echo ${S} ${Y:=1} ${Z:-2}
		`,
			command: &cmd.ShellCommand{
				Exports: map[string]*cmd.ShellValue{"Y": nil},
				Locals:  map[string]*cmd.ShellValue{"S": nil, "Z": lit("2")},
			},
			scenarios: []scenario{
				{
					s: `export Y
echo ${S} ${Y:=1} ${Z:-2}
		`,
					locals: map[string]*cmd.ShellValue{},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			command, err := cmd.NewShellCommand(c.source)
			if err != nil {
				t.Fatal(err)
			}

			// Copy these over for now
			c.command.File = command.File
			c.command.LocalDeclarations = command.LocalDeclarations

			assert.Equal(t, command, c.command)

			scenarios := c.scenarios
			if scenarios == nil {
				scenarios = []scenario{
					{
						s:      c.source,
						locals: map[string]*cmd.ShellValue{},
					},
				}
			}

			for _, r := range scenarios {
				s, err := command.Render(r.locals)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t,
					strings.Trim(s, "\n\t")+"\n",
					strings.Trim(r.s, "\n\t")+"\n",
				)
			}
		})
	}
}
