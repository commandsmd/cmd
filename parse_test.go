package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cmd "github.com/commandsmd/cmd"
)

func TestParseCommandDefinitions(t *testing.T) {
	var cases = []struct {
		name   string
		source string
		defs   []cmd.CommandDefinition
	}{}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defs, err := cmd.ParseCommandDefinitions([]byte(c.source))
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, defs, c.defs)
		})
	}
}
