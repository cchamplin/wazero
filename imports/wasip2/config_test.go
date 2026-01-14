// Copyright 2024 Tetrate
// SPDX-License-Identifier: Apache-2.0

package wasip2

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	require.NotNil(t, config)
	require.NotNil(t, config.Stdin())
	require.NotNil(t, config.Stdout())
	require.NotNil(t, config.Stderr())
	require.NotNil(t, config.Environ())
	require.NotNil(t, config.Args())
}

func TestConfigWithStdio(t *testing.T) {
	stdin := bytes.NewReader([]byte("hello"))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	config := NewConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr)

	require.Equal(t, stdin, config.Stdin())
	require.Equal(t, stdout, config.Stdout())
	require.Equal(t, stderr, config.Stderr())
}

func TestConfigWithEnviron(t *testing.T) {
	config := NewConfig().
		WithEnviron([]string{"FOO=bar", "BAZ=qux"})

	require.Equal(t, []string{"FOO=bar", "BAZ=qux"}, config.Environ())
}

func TestConfigWithArgs(t *testing.T) {
	config := NewConfig().
		WithArgs([]string{"prog", "arg1", "arg2"})

	require.Equal(t, []string{"prog", "arg1", "arg2"}, config.Args())
}

func TestConfigWithPreopens(t *testing.T) {
	config := NewConfig().
		WithPreopen("/guest", "/host/path").
		WithPreopen("/tmp", "/var/tmp")

	preopens := config.Preopens()
	require.Equal(t, 2, len(preopens))
	require.Equal(t, "/host/path", preopens["/guest"])
	require.Equal(t, "/var/tmp", preopens["/tmp"])
}
