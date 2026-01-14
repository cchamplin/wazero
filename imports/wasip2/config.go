// Copyright 2024 Tetrate
// SPDX-License-Identifier: Apache-2.0

package wasip2

import (
	"io"
	"os"
)

// Config configures WASI Preview 2 behavior.
type Config struct {
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	environ func() []string
	args    func() []string
	preopens map[string]string

	// Feature flags
	allowNetwork bool
	allowHTTP    bool
}

// NewConfig creates a new config with no defaults set.
func NewConfig() *Config {
	return &Config{
		preopens: make(map[string]string),
	}
}

// DefaultConfig returns config backed by os package defaults.
func DefaultConfig() *Config {
	return &Config{
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		environ: os.Environ,
		args:    func() []string { return os.Args },
		preopens: make(map[string]string),
		allowNetwork: true,
		allowHTTP:    true,
	}
}

// WithStdin sets the reader for stdin.
func (c *Config) WithStdin(r io.Reader) *Config {
	c.stdin = r
	return c
}

// WithStdout sets the writer for stdout.
func (c *Config) WithStdout(w io.Writer) *Config {
	c.stdout = w
	return c
}

// WithStderr sets the writer for stderr.
func (c *Config) WithStderr(w io.Writer) *Config {
	c.stderr = w
	return c
}

// WithEnviron sets the environment variables as "KEY=value" strings.
func (c *Config) WithEnviron(environ []string) *Config {
	c.environ = func() []string { return environ }
	return c
}

// WithArgs sets the command-line arguments.
func (c *Config) WithArgs(args []string) *Config {
	c.args = func() []string { return args }
	return c
}

// WithPreopen maps a guest path to a host path for filesystem access.
func (c *Config) WithPreopen(guestPath, hostPath string) *Config {
	c.preopens[guestPath] = hostPath
	return c
}

// WithNetwork enables or disables network access.
func (c *Config) WithNetwork(allow bool) *Config {
	c.allowNetwork = allow
	return c
}

// WithHTTP enables or disables HTTP access.
func (c *Config) WithHTTP(allow bool) *Config {
	c.allowHTTP = allow
	return c
}

// Stdin returns the configured stdin reader.
func (c *Config) Stdin() io.Reader { return c.stdin }

// Stdout returns the configured stdout writer.
func (c *Config) Stdout() io.Writer { return c.stdout }

// Stderr returns the configured stderr writer.
func (c *Config) Stderr() io.Writer { return c.stderr }

// Environ returns the configured environment variables.
func (c *Config) Environ() []string {
	if c.environ == nil {
		return nil
	}
	return c.environ()
}

// Args returns the configured command-line arguments.
func (c *Config) Args() []string {
	if c.args == nil {
		return nil
	}
	return c.args()
}

// Preopens returns the configured guest-to-host path mappings.
func (c *Config) Preopens() map[string]string { return c.preopens }

// AllowNetwork returns whether network access is allowed.
func (c *Config) AllowNetwork() bool { return c.allowNetwork }

// AllowHTTP returns whether HTTP access is allowed.
func (c *Config) AllowHTTP() bool { return c.allowHTTP }
