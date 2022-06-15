package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type ShellCommand struct {
	Exports map[string]*ShellValue
	Locals  map[string]*ShellValue

	LocalDeclarations map[string]*syntax.ParamExp

	File *syntax.File
}

func (c *ShellCommand) Print() (string, error) {
	var b = bytes.Buffer{}
	err := syntax.NewPrinter().Print(&b, c.File)
	return b.String(), err
}

func (c *ShellCommand) Render(locals map[string]*ShellValue) (string, error) {
	s, err := c.Print()
	if err != nil {
		return "", err
	}

	if len(locals) == 0 {
		return s, nil
	}

	// Reparse
	c2, err := NewShellCommand(s)
	if err != nil {
		return "", err
	}

	for name, value := range locals {
		node, ok := c2.LocalDeclarations[name]
		if !ok {
			return "", fmt.Errorf("unknown local: %q", name)
		}

		if value.Expression != "" {
			return "", fmt.Errorf("local with expression values are not yet supported. local: %q", name)
		}

		node.Exp = &syntax.Expansion{
			Op: syntax.AssignUnsetOrNull,
			Word: &syntax.Word{
				Parts: []syntax.WordPart{
					&syntax.Lit{
						ValuePos: syntax.NewPos(0, 0, 0),
						ValueEnd: syntax.NewPos(0, 0, 0),
						Value:    value.Literal,
					},
				},
			},
		}
	}

	return c2.Print()
}

type ShellValue struct {
	Literal    string
	Expression string
}

func asShellValue(w *syntax.Word) (*ShellValue, error) {
	if w == nil {
		return nil, nil
	}

	literal := w.Lit()
	expression := ""
	if literal == "" {
		var b = bytes.Buffer{}
		if err := syntax.NewPrinter().Print(&b, w); err != nil {
			return nil, err
		}

		expression = b.String()
	}

	return &ShellValue{
		Literal:    literal,
		Expression: expression,
	}, nil
}

func NewShellCommand(shell string) (*ShellCommand, error) {
	r := strings.NewReader(shell)
	f, err := syntax.NewParser().Parse(r, "")
	if err != nil {
		return nil, err
	}

	exports := map[string]*ShellValue{}
	locals := map[string]*ShellValue{}

	// Track where locals are defined so we can rewrite them with values as needed.
	variableReferenceNodes := map[string]*syntax.ParamExp{}

	variableReferences := map[string]bool{}
	variableReferenceDefaults := map[string]*ShellValue{}

	isAlpha := func(ch byte) bool {
		return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
	}

	syntax.Walk(f, func(n syntax.Node) bool {
		// fmt.Printf("%#v\n", n)
		switch x := n.(type) {
		case *syntax.FuncDecl:
			// Don't enter function declarations. Only consider top-level expressions.
			return false
		case *syntax.Assign:
			name := x.Name.Value
			if !x.Naked {
				l, err := asShellValue(x.Value)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %s\n", err)
				} else {
					locals[name] = l
				}
			}

		case *syntax.DeclClause:
			switch x.Variant.Value {
			case "export":
				for _, arg := range x.Args {
					name := arg.Name.Value
					if arg.Naked {
						exports[name] = nil
					} else {
						l, err := asShellValue(arg.Value)
						if err != nil {
							fmt.Fprintf(os.Stderr, "warning: %s\n", err)
						} else {
							exports[name] = l
						}
					}
				}
			}
			return false
		case *syntax.CallExpr:
			syntax.Walk(f, func(n syntax.Node) bool {
				switch x := n.(type) {
				case *syntax.ParamExp:
					if x.Param != nil && x.Param.Value != "" {
						name := x.Param.Value
						ch := name[0]
						if _, ok := variableReferenceNodes[name]; !ok {
							variableReferenceNodes[name] = x
						}
						if isAlpha(ch) {
							if x.Exp != nil {
								s, err := asShellValue(x.Exp.Word)
								if err != nil {
									fmt.Fprintf(os.Stderr, "warning: %s\n", err)
								}
								if s != nil {
									variableReferenceDefaults[name] = s
								}
							} else {
								if _, ok := variableReferenceDefaults[name]; !ok {
									variableReferenceDefaults[name] = nil
								}
							}
						}
					}
				}

				return true
			})
		case *syntax.ParamExp:
			if x.Param != nil && x.Param.Value != "" {
				name := x.Param.Value
				ch := name[0]
				if isAlpha(ch) {
					variableReferences[name] = true
				}
			}
		}
		return true
	})

	// Promote referenced variables that aren't declared as exported to
	// be locals
	for v := range variableReferences {
		if _, isExport := exports[v]; !isExport {
			if defaultValue, isLocal := locals[v]; !isLocal || defaultValue == nil {
				locals[v] = variableReferenceDefaults[v]
			}
		}
	}

	return &ShellCommand{
		Exports: exports,
		Locals:  locals,

		LocalDeclarations: variableReferenceNodes,
		File:              f,
	}, nil
}
