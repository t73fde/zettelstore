//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

package kernel

import (
	"fmt"
	"io"
	"maps"
	"os"
	"runtime/metrics"
	"slices"
	"strconv"
	"strings"

	"zettelstore.de/z/internal/logger"
	"zettelstore.de/z/strfun"
)

type cmdSession struct {
	w        io.Writer
	kern     *Kernel
	echo     bool
	header   bool
	colwidth int
	eol      []byte
}

func (sess *cmdSession) initialize(w io.Writer, kern *Kernel) {
	sess.w = w
	sess.kern = kern
	sess.header = true
	sess.colwidth = 80
	sess.eol = []byte{'\n'}
}

func (sess *cmdSession) executeLine(line string) bool {
	if sess.echo {
		sess.println(line)
	}
	cmd, args := splitLine(line)
	if c, ok := commands[cmd]; ok {
		return c.Func(sess, cmd, args)
	}
	if cmd == "help" {
		return cmdHelp(sess, cmd, args)
	}
	sess.println("Unknown command:", cmd, strings.Join(args, " "))
	sess.println("-- Enter 'help' go get a list of valid commands.")
	return true
}

func (sess *cmdSession) println(args ...string) {
	if len(args) > 0 {
		_, _ = io.WriteString(sess.w, args[0])
		for _, arg := range args[1:] {
			_, _ = io.WriteString(sess.w, " ")
			_, _ = io.WriteString(sess.w, arg)
		}
	}
	_, _ = sess.w.Write(sess.eol)
}

func (sess *cmdSession) usage(cmd, val string) {
	sess.println("Usage:", cmd, val)
}

func (sess *cmdSession) printTable(table [][]string) {
	maxLen := sess.calcMaxLen(table)
	if len(maxLen) == 0 {
		return
	}
	if sess.header {
		sess.printRow(table[0], maxLen, "|=", " | ", ' ')
		hLine := make([]string, len(table[0]))
		sess.printRow(hLine, maxLen, "|%", "-+-", '-')
	}

	for _, row := range table[1:] {
		sess.printRow(row, maxLen, "| ", " | ", ' ')
	}
}

func (sess *cmdSession) calcMaxLen(table [][]string) []int {
	maxLen := make([]int, 0)
	for _, row := range table {
		for colno, column := range row {
			if colno >= len(maxLen) {
				maxLen = append(maxLen, 0)
			}
			colLen := strfun.Length(column)
			if colLen <= maxLen[colno] {
				continue
			}
			maxLen[colno] = min(colLen, sess.colwidth)
		}
	}
	return maxLen
}

func (sess *cmdSession) printRow(row []string, maxLen []int, prefix, delim string, pad rune) {
	for colno, column := range row {
		_, _ = io.WriteString(sess.w, prefix)
		prefix = delim
		_, _ = io.WriteString(sess.w, strfun.JustifyLeft(column, maxLen[colno], pad))
	}
	_, _ = sess.w.Write(sess.eol)
}

func splitLine(line string) (string, []string) {
	s := strings.Fields(line)
	if len(s) == 0 {
		return "", nil
	}
	return strings.ToLower(s[0]), s[1:]
}

type command struct {
	Text string
	Func func(sess *cmdSession, cmd string, args []string) bool
}

var commands = map[string]command{
	"": {"", func(*cmdSession, string, []string) bool { return true }},
	"bye": {
		"end this session",
		func(*cmdSession, string, []string) bool { return false },
	},
	"config": {"show configuration keys", cmdConfig},
	"crlf": {
		"toggle crlf mode",
		func(sess *cmdSession, _ string, _ []string) bool {
			if len(sess.eol) == 1 {
				sess.eol = []byte{'\r', '\n'}
				sess.println("crlf is on")
			} else {
				sess.eol = []byte{'\n'}
				sess.println("crlf is off")
			}
			return true
		},
	},
	"dump-index":   {"writes the content of the index", cmdDumpIndex},
	"dump-recover": {"show data of last recovery", cmdDumpRecover},
	"echo": {
		"toggle echo mode",
		func(sess *cmdSession, _ string, _ []string) bool {
			sess.echo = !sess.echo
			if sess.echo {
				sess.println("echo is on")
			} else {
				sess.println("echo is off")
			}
			return true
		},
	},
	"end-profile": {"stop profiling", cmdEndProfile},
	"env":         {"show environment values", cmdEnvironment},
	"get-config":  {"show current configuration data", cmdGetConfig},
	"header": {
		"toggle table header",
		func(sess *cmdSession, _ string, _ []string) bool {
			sess.header = !sess.header
			if sess.header {
				sess.println("header are on")
			} else {
				sess.println("header are off")
			}
			return true
		},
	},
	"log-level":   {"get/set log level", cmdLogLevel},
	"metrics":     {"show Go runtime metrics", cmdMetrics},
	"next-config": {"show next configuration data", cmdNextConfig},
	"profile":     {"start profiling", cmdProfile},
	"refresh":     {"refresh box data", cmdRefresh},
	"restart":     {"restart service", cmdRestart},
	"services":    {"show available services", cmdServices},
	"set-config":  {"set next configuration data", cmdSetConfig},
	"shutdown": {
		"shutdown Zettelstore",
		func(sess *cmdSession, _ string, _ []string) bool { sess.kern.Shutdown(false); return false },
	},
	"start": {"start service", cmdStart},
	"stat":  {"show service statistics", cmdStat},
	"stop":  {"stop service", cmdStop},
}

func cmdHelp(sess *cmdSession, _ string, _ []string) bool {
	table := [][]string{{"Command", "Description"}}
	for _, cmd := range slices.Sorted(maps.Keys(commands)) {
		table = append(table, []string{cmd, commands[cmd].Text})
	}
	sess.printTable(table)
	return true
}

func cmdConfig(sess *cmdSession, cmd string, args []string) bool {
	srvnum, ok := lookupService(sess, cmd, args)
	if !ok {
		return true
	}
	srv := sess.kern.srvs[srvnum].srv
	table := [][]string{{"Key", "Description"}}
	for _, kd := range srv.ConfigDescriptions() {
		table = append(table, []string{kd.Key, kd.Descr})
	}
	sess.printTable(table)
	return true
}
func cmdGetConfig(sess *cmdSession, _ string, args []string) bool {
	showConfig(sess, args,
		listCurConfig, func(srv service, key string) any { return srv.GetCurConfig(key) })
	return true
}
func cmdNextConfig(sess *cmdSession, _ string, args []string) bool {
	showConfig(sess, args,
		listNextConfig, func(srv service, key string) any { return srv.GetNextConfig(key) })
	return true
}
func showConfig(sess *cmdSession, args []string,
	listConfig func(*cmdSession, service), getConfig func(service, string) any) {

	if len(args) == 0 {
		keys := make([]int, 0, len(sess.kern.srvs))
		for k := range sess.kern.srvs {
			keys = append(keys, int(k))
		}
		slices.Sort(keys)
		for i, k := range keys {
			if i > 0 {
				sess.println()
			}
			srvD := sess.kern.srvs[Service(k)]
			sess.println("%% Service", srvD.name)
			listConfig(sess, srvD.srv)

		}
		return
	}
	srvD, found := getService(sess, args[0])
	if !found {
		return
	}
	if len(args) == 1 {
		listConfig(sess, srvD.srv)
		return
	}
	val := getConfig(srvD.srv, args[1])
	if val == nil {
		sess.println("Unknown key", args[1], "for service", args[0])
		return
	}
	sess.println(fmt.Sprintf("%v", val))
}
func listCurConfig(sess *cmdSession, srv service) {
	listConfig(sess, func() []KeyDescrValue { return srv.GetCurConfigList(true) })
}
func listNextConfig(sess *cmdSession, srv service) {
	listConfig(sess, srv.GetNextConfigList)
}
func listConfig(sess *cmdSession, getConfigList func() []KeyDescrValue) {
	l := getConfigList()
	table := [][]string{{"Key", "Value", "Description"}}
	for _, kdv := range l {
		table = append(table, []string{kdv.Key, kdv.Value, kdv.Descr})
	}
	sess.printTable(table)
}

func cmdSetConfig(sess *cmdSession, cmd string, args []string) bool {
	if len(args) < 3 {
		sess.usage(cmd, "SERVICE KEY VALUE")
		return true
	}
	srvD, found := getService(sess, args[0])
	if !found {
		return true
	}
	key := args[1]
	newValue := strings.Join(args[2:], " ")
	if err := srvD.srv.SetConfig(key, newValue); err == nil {
		sess.kern.dlogger.Mandatory().Str("key", key).Str("value", newValue).Msg("Update system configuration")
	} else {
		sess.println("Unable to set key", args[1], "to value", newValue, "because:", err.Error())
	}
	return true
}

func cmdServices(sess *cmdSession, _ string, _ []string) bool {
	table := [][]string{{"Service", "Status"}}
	for _, name := range sortedServiceNames(sess) {
		if sess.kern.srvNames[name].srv.IsStarted() {
			table = append(table, []string{name, "started"})
		} else {
			table = append(table, []string{name, "stopped"})
		}
	}
	sess.printTable(table)
	return true
}

func cmdStart(sess *cmdSession, cmd string, args []string) bool {
	srvnum, ok := lookupService(sess, cmd, args)
	if !ok {
		return true
	}
	err := sess.kern.doStartService(srvnum)
	if err != nil {
		sess.println(err.Error())
	}
	return true
}

func cmdRestart(sess *cmdSession, cmd string, args []string) bool {
	srvnum, ok := lookupService(sess, cmd, args)
	if !ok {
		return true
	}
	err := sess.kern.restartService(srvnum)
	if err != nil {
		sess.println(err.Error())
	}
	return true
}

func cmdStop(sess *cmdSession, cmd string, args []string) bool {
	if srvnum, ok := lookupService(sess, cmd, args); ok {
		sess.kern.doStopService(srvnum)
	}
	return true
}

func cmdStat(sess *cmdSession, cmd string, args []string) bool {
	if len(args) == 0 {
		sess.usage(cmd, "SERVICE")
		return true
	}
	srvD, ok := getService(sess, args[0])
	if !ok {
		return true
	}
	kvl := srvD.srv.GetStatistics()
	if len(kvl) == 0 {
		return true
	}
	table := [][]string{{"Key", "Value"}}
	for _, kv := range kvl {
		table = append(table, []string{kv.Key, kv.Value})
	}
	sess.printTable(table)
	return true
}

func cmdLogLevel(sess *cmdSession, _ string, args []string) bool {
	kern := sess.kern
	if len(args) == 0 {
		// Write log levels
		level := kern.dlogger.Level()
		table := [][]string{
			{"Service", "Level", "Name"},
			{"kernel", strconv.Itoa(int(level)), level.String()},
		}
		for _, name := range sortedServiceNames(sess) {
			level = kern.srvNames[name].srv.GetLogger().Level()
			table = append(table, []string{name, strconv.Itoa(int(level)), level.String()})
		}
		sess.printTable(table)
		return true
	}
	var l *logger.DLogger
	name := args[0]
	if name == "kernel" {
		l = kern.dlogger
	} else {
		srvD, ok := getService(sess, name)
		if !ok {
			return true
		}
		l = srvD.srv.GetLogger()
	}

	if len(args) == 1 {
		level := l.Level()
		sess.println(strconv.Itoa(int(level)), level.String())
		return true
	}

	level := args[1]
	uval, err := strconv.ParseUint(level, 10, 8)
	lv := logger.DLevel(uval)
	if err != nil || !lv.IsValid() {
		lv = logger.DParseLevel(level)
	}
	if !lv.IsValid() {
		sess.println("Invalid level:", level)
		return true
	}
	kern.dlogger.Mandatory().Str("name", name).Str("level", lv.String()).Msg("Update log level")
	l.SetLevel(lv)
	return true
}

func lookupService(sess *cmdSession, cmd string, args []string) (Service, bool) {
	if len(args) == 0 {
		sess.usage(cmd, "SERVICE")
		return 0, false
	}
	srvD, ok := getService(sess, args[0])
	if !ok {
		return 0, false
	}
	return srvD.srvnum, true
}

func cmdProfile(sess *cmdSession, _ string, args []string) bool {
	var profileName string
	if len(args) < 1 {
		profileName = ProfileCPU
	} else {
		profileName = args[0]
	}
	var fileName string
	if len(args) < 2 {
		fileName = profileName + ".prof"
	} else {
		fileName = args[1]
	}
	kern := sess.kern
	if err := kern.doStartProfiling(profileName, fileName); err != nil {
		sess.println("Error:", err.Error())
	} else {
		kern.dlogger.Mandatory().Str("profile", profileName).Str("file", fileName).Msg("Start profiling")
	}
	return true
}
func cmdEndProfile(sess *cmdSession, _ string, _ []string) bool {
	kern := sess.kern
	err := kern.doStopProfiling()
	if err != nil {
		sess.println("Error:", err.Error())
	}
	kern.dlogger.Mandatory().Err(err).Msg("Stop profiling")
	return true
}

func cmdMetrics(sess *cmdSession, _ string, _ []string) bool {
	var samples []metrics.Sample
	all := metrics.All()
	for _, d := range all {
		if d.Kind == metrics.KindFloat64Histogram {
			continue
		}
		samples = append(samples, metrics.Sample{Name: d.Name})
	}
	metrics.Read(samples)

	table := [][]string{{"Value", "Description"}}
	i := 0
	for _, d := range all {
		if d.Kind == metrics.KindFloat64Histogram {
			continue
		}
		descr := d.Description
		if pos := strings.IndexByte(descr, '.'); pos > 0 {
			descr = descr[:pos]
		}
		value := samples[i].Value
		i++
		var sVal string
		switch value.Kind() {
		case metrics.KindUint64:
			sVal = strconv.FormatUint(value.Uint64(), 10)
		case metrics.KindFloat64:
			sVal = fmt.Sprintf("%v", value.Float64())
		case metrics.KindFloat64Histogram:
			sVal = "(Histogramm)"
		case metrics.KindBad:
			sVal = "BAD"
		default:
			sVal = fmt.Sprintf("(unexpected metric kind: %v)", value.Kind())
		}
		table = append(table, []string{sVal, descr})
	}
	sess.printTable(table)
	return true
}

func cmdDumpIndex(sess *cmdSession, _ string, _ []string) bool {
	sess.kern.dumpIndex(sess.w)
	return true
}

func cmdRefresh(sess *cmdSession, _ string, _ []string) bool {
	kern := sess.kern
	kern.dlogger.Mandatory().Msg("Refresh")
	if err := kern.box.Refresh(); err != nil {
		kern.dlogger.Error().Err(err).Msg("refresh")
	}
	return true
}

func cmdDumpRecover(sess *cmdSession, cmd string, args []string) bool {
	if len(args) == 0 {
		sess.usage(cmd, "RECOVER")
		sess.println("-- A valid value for RECOVER can be obtained via 'stat core'.")
		return true
	}
	lines := sess.kern.core.RecoverLines(args[0])
	if len(lines) == 0 {
		return true
	}
	for _, line := range lines {
		sess.println(line)
	}
	return true
}

func cmdEnvironment(sess *cmdSession, _ string, _ []string) bool {
	workDir, err := os.Getwd()
	if err != nil {
		workDir = err.Error()
	}
	execName, err := os.Executable()
	if err != nil {
		execName = err.Error()
	}
	envs := os.Environ()
	slices.Sort(envs)

	table := [][]string{
		{"Key", "Value"},
		{"workdir", workDir},
		{"executable", execName},
	}
	for _, env := range envs {
		if pos := strings.IndexByte(env, '='); pos >= 0 && pos < len(env) {
			table = append(table, []string{env[:pos], env[pos+1:]})
		}
	}
	sess.printTable(table)
	return true
}

func sortedServiceNames(sess *cmdSession) []string {
	return slices.Sorted(maps.Keys(sess.kern.srvNames))
}

func getService(sess *cmdSession, name string) (serviceData, bool) {
	srvD, found := sess.kern.srvNames[name]
	if !found {
		sess.println("Unknown service", name)
	}
	return srvD, found
}
