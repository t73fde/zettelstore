//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

package cmd

import (
	"flag"
	"log/slog"
	"maps"
	"slices"
)

// Command stores information about commands / sub-commands.
type Command struct {
	Name       string              // command name as it appears on the command line
	Func       CommandFunc         // function that executes a command
	Simple     bool                // Operate in simple-mode
	Boxes      bool                // if true then boxes will be set up
	Header     bool                // Print a heading on startup
	LineServer bool                // Start admin line server
	SetFlags   func(*flag.FlagSet) // function to set up flag.FlagSet
	flags      *flag.FlagSet       // flags that belong to the command
}

// CommandFunc is the function that executes the command.
// It accepts the parsed command line parameters.
// It returns the exit code and an error.
type CommandFunc func(*flag.FlagSet) (int, error)

// GetFlags return the flag.FlagSet defined for the command.
func (c *Command) GetFlags() *flag.FlagSet { return c.flags }

var commands = make(map[string]Command)

// RegisterCommand registers the given command.
func RegisterCommand(cmd Command) {
	if cmd.Name == "" || cmd.Func == nil {
		panic("Required command values missing")
	}
	if _, ok := commands[cmd.Name]; ok {
		panic("Command already registered: " + cmd.Name)
	}
	cmd.flags = flag.NewFlagSet(cmd.Name, flag.ExitOnError)
	cmd.flags.String("l", slog.LevelInfo.String(), "log level specification")

	if cmd.SetFlags != nil {
		cmd.SetFlags(cmd.flags)
	}
	commands[cmd.Name] = cmd
}

// Get returns the command identified by the given name and a bool to signal success.
func Get(name string) (Command, bool) {
	cmd, ok := commands[name]
	return cmd, ok
}

// List returns a sorted list of all registered command names.
func List() []string { return slices.Sorted(maps.Keys(commands)) }
