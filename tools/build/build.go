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

// Package main provides a command to build and run the software.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/input"
	"zettelstore.de/z/strfun"
	"zettelstore.de/z/tools"
	"zettelstore.de/z/zettel/meta"
)

func readVersionFile() (string, error) {
	content, err := os.ReadFile("VERSION")
	if err != nil {
		return "", err
	}
	return strings.TrimFunc(string(content), func(r rune) bool {
		return r <= ' '
	}), nil
}

func getVersion() string {
	base, err := readVersionFile()
	if err != nil {
		base = "dev"
	}
	return base
}

var dirtyPrefixes = []string{
	"DELETED ", "ADDED ", "UPDATED ", "CONFLICT ", "EDITED ", "RENAMED ", "EXTRA "}

const dirtySuffix = "-dirty"

func readFossilDirty() (string, error) {
	s, err := tools.ExecuteCommand(nil, "fossil", "status", "--differ")
	if err != nil {
		return "", err
	}
	for _, line := range strfun.SplitLines(s) {
		for _, prefix := range dirtyPrefixes {
			if strings.HasPrefix(line, prefix) {
				return dirtySuffix, nil
			}
		}
	}
	return "", nil
}

func getFossilDirty() string {
	fossil, err := readFossilDirty()
	if err != nil {
		return ""
	}
	return fossil
}

func cmdBuild() error {
	return doBuild(tools.EnvDirectProxy, getVersion(), "bin/zettelstore")
}

func doBuild(env []string, version, target string) error {
	env = append(env, "CGO_ENABLED=0")
	env = append(env, tools.EnvGoVCS...)
	out, err := tools.ExecuteCommand(
		env,
		"go", "build",
		"-tags", "osusergo,netgo",
		"-trimpath",
		"-ldflags", fmt.Sprintf("-X main.version=%v -w", version),
		"-o", target,
		"zettelstore.de/z/cmd/zettelstore",
	)
	if err != nil {
		return err
	}
	if len(out) > 0 {
		fmt.Println(out)
	}
	return nil
}

func cmdHelp() {
	fmt.Println(`Usage: go run tools/build/build.go [-v] COMMAND

Options:
  -v       Verbose output.

Commands:
  build     Build the software for local computer.
  help      Output this text.
  manual    Create a ZIP file with all manual zettel
  release   Create the software for various platforms and put them in
            appropriate named ZIP files.
  version   Print the current version of the software.

All commands can be abbreviated as long as they remain unique.`)
}

func main() {
	flag.BoolVar(&tools.Verbose, "v", false, "Verbose output")
	flag.Parse()
	var err error
	args := flag.Args()
	if len(args) < 1 {
		cmdHelp()
	} else {
		switch args[0] {
		case "b", "bu", "bui", "buil", "build":
			err = cmdBuild()
		case "m", "ma", "man", "manu", "manua", "manual":
			err = cmdManual()
		case "r", "re", "rel", "rele", "relea", "releas", "release":
			err = cmdRelease()
		case "v", "ve", "ver", "vers", "versi", "versio", "version":
			fmt.Print(getVersion())
		case "h", "he", "hel", "help":
			cmdHelp()
		default:
			fmt.Fprintf(os.Stderr, "Unknown command %q\n", args[0])
			cmdHelp()
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// --- manual

func cmdManual() error {
	base := getReleaseVersionData()
	return createManualZip(".", base)
}

func createManualZip(path, base string) error {
	manualPath := filepath.Join("docs", "manual")
	entries, err := os.ReadDir(manualPath)
	if err != nil {
		return err
	}
	zipName := filepath.Join(path, "manual-"+base+".zip")
	zipFile, err := os.OpenFile(zipName, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, entry := range entries {
		if err = createManualZipEntry(manualPath, entry, zipWriter); err != nil {
			return err
		}
	}
	return nil
}

const versionZid = "00001000000001"

func createManualZipEntry(path string, entry fs.DirEntry, zipWriter *zip.Writer) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	fh, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	name := entry.Name()
	fh.Name = name
	fh.Method = zip.Deflate
	w, err := zipWriter.CreateHeader(fh)
	if err != nil {
		return err
	}
	manualFile, err := os.Open(filepath.Join(path, name))
	if err != nil {
		return err
	}
	defer manualFile.Close()

	if name != versionZid+".zettel" {
		_, err = io.Copy(w, manualFile)
		return err
	}

	data, err := io.ReadAll(manualFile)
	if err != nil {
		return err
	}
	inp := input.NewInput(data)
	m := meta.NewFromInput(id.MustParse(versionZid), inp)
	m.SetNow(api.KeyModified)

	var buf bytes.Buffer
	if _, err = fmt.Fprintf(&buf, "id: %s\n", versionZid); err != nil {
		return err
	}
	if _, err = m.WriteComputed(&buf); err != nil {
		return err
	}
	version := getVersion()
	if _, err = fmt.Fprintf(&buf, "\n%s", version); err != nil {
		return err
	}
	_, err = io.Copy(w, &buf)
	return err
}

//--- release

func cmdRelease() error {
	if err := tools.Check(true); err != nil {
		return err
	}
	base := getReleaseVersionData()
	releases := []struct {
		arch string
		os   string
		env  []string
		name string
	}{
		{"amd64", "linux", nil, "zettelstore"},
		{"arm", "linux", []string{"GOARM=6"}, "zettelstore"},
		{"arm64", "darwin", nil, "zettelstore"},
		{"amd64", "darwin", nil, "zettelstore"},
		{"amd64", "windows", nil, "zettelstore.exe"},
		{"arm64", "android", nil, "zettelstore"},
	}
	for _, rel := range releases {
		env := append([]string{}, rel.env...)
		env = append(env, "GOARCH="+rel.arch, "GOOS="+rel.os)
		env = append(env, tools.EnvDirectProxy...)
		env = append(env, tools.EnvGoVCS...)
		zsName := filepath.Join("releases", rel.name)
		if err := doBuild(env, base, zsName); err != nil {
			return err
		}
		zipName := fmt.Sprintf("zettelstore-%v-%v-%v.zip", base, rel.os, rel.arch)
		if err := createReleaseZip(zsName, zipName, rel.name); err != nil {
			return err
		}
		if err := os.Remove(zsName); err != nil {
			return err
		}
	}
	return createManualZip("releases", base)
}

func getReleaseVersionData() string {
	if fossil := getFossilDirty(); fossil != "" {
		fmt.Fprintln(os.Stderr, "Warning: releasing a dirty version")
	}
	base := getVersion()
	if strings.HasSuffix(base, "dev") {
		return base[:len(base)-3] + "preview-" + time.Now().Local().Format("20060102")
	}
	return base
}

func createReleaseZip(zsName, zipName, fileName string) error {
	zipFile, err := os.OpenFile(filepath.Join("releases", zipName), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	zw := zip.NewWriter(zipFile)
	defer zw.Close()
	err = addFileToZip(zw, zsName, fileName)
	if err != nil {
		return err
	}
	err = addFileToZip(zw, "LICENSE.txt", "LICENSE.txt")
	if err != nil {
		return err
	}
	err = addFileToZip(zw, "docs/readmezip.txt", "README.txt")
	return err
}

func addFileToZip(zipFile *zip.Writer, filepath, filename string) error {
	zsFile, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer zsFile.Close()
	stat, err := zsFile.Stat()
	if err != nil {
		return err
	}
	fh, err := zip.FileInfoHeader(stat)
	if err != nil {
		return err
	}
	fh.Name = filename
	fh.Method = zip.Deflate
	w, err := zipFile.CreateHeader(fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, zsFile)
	return err
}
