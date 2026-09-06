//go:build unit

package main

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdaterRejectsPositionalCommandsBeforeStartup(t *testing.T) {
	for _, args := range [][]string{{"status"}, {"status", "--help"}} {
		cmd := exec.Command(os.Args[0], "-test.run=TestUpdaterPositionalCommandHelper")
		cmd.Env = append(os.Environ(), "SUB2API_UPDATER_POSITIONAL_TEST="+strings.Join(args, "\x1f"))
		output, err := cmd.CombinedOutput()
		require.Error(t, err)
		require.Contains(t, string(output), "unexpected positional arguments")
		require.NotContains(t, string(output), "load updater policy")
	}
}

func TestUpdaterPositionalCommandHelper(t *testing.T) {
	raw := os.Getenv("SUB2API_UPDATER_POSITIONAL_TEST")
	if raw == "" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("updater", flag.ContinueOnError)
	os.Args = append([]string{"sub2api-rework-updater"}, strings.Split(raw, "\x1f")...)
	main()
}
