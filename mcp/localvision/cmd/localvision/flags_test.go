package main

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseFlags maps the stdlib flag-parse result to an exit code. -h/--help must
// exit 0 (matching the top-level `localvision --help`), not 2. These tests pin
// that mapping (Tier-1 #5 regression).

func newTestFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard) // swallow usage output during the test
	return fs
}

func TestParseFlags_HelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		t.Run(args[0], func(t *testing.T) {
			fs := newTestFlagSet("test")
			code, done := parseFlags(fs, args)
			assert.True(t, done, "-h/--help should signal the caller to return")
			assert.Equal(t, exitOK, code, "-h/--help must exit 0, not 2")
		})
	}
}

func TestParseFlags_BadArgsExitTwo(t *testing.T) {
	fs := newTestFlagSet("test")
	code, done := parseFlags(fs, []string{"--no-such-flag"})
	assert.True(t, done, "a bad flag should signal the caller to return")
	assert.Equal(t, exitBadArgs, code, "a genuine parse error must exit 2")
}

func TestParseFlags_SuccessContinues(t *testing.T) {
	fs := newTestFlagSet("test")
	var verbose bool
	fs.BoolVar(&verbose, "verbose", false, "")
	code, done := parseFlags(fs, []string{"-verbose"})
	assert.False(t, done, "a successful parse should let the caller continue")
	assert.Equal(t, 0, code)
	require.True(t, verbose, "flag should still be parsed on success")
}
