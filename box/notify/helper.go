//-----------------------------------------------------------------------------
// Copyright (c) 2021-2022 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package notify

import (
	"archive/zip"
	"os"

	"zettelstore.de/z/logger"
)

// MakeMetaFilename builds the name of the file containing metadata.
func MakeMetaFilename(basename string) string {
	return basename //+ ".meta"
}

// EntryFetcher return a list of (file) names of an directory.
type EntryFetcher interface {
	Fetch() ([]string, error)
}

type dirPathFetcher struct {
	dirPath string
}

func newDirPathFetcher(dirPath string) EntryFetcher { return &dirPathFetcher{dirPath} }

func (dpf *dirPathFetcher) Fetch() ([]string, error) {
	entries, err := os.ReadDir(dpf.dirPath)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if info, err1 := entry.Info(); err1 != nil || !info.Mode().IsRegular() {
			continue
		}
		result = append(result, entry.Name())
	}
	return result, nil
}

type zipPathFetcher struct {
	zipPath string
}

func newZipPathFetcher(zipPath string) EntryFetcher { return &zipPathFetcher{zipPath} }

func (zpf *zipPathFetcher) Fetch() ([]string, error) {
	reader, err := zip.OpenReader(zpf.zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	result := make([]string, 0, len(reader.File))
	for _, f := range reader.File {
		result = append(result, f.Name)
	}
	return result, nil
}

// listDirElements write all files within the directory path as events.
func listDirElements(log *logger.Logger, fetcher EntryFetcher, events chan<- Event, done <-chan struct{}) bool {
	select {
	case events <- Event{Op: Make}:
	case <-done:
		return false
	}
	entries, err := fetcher.Fetch()
	if err != nil {
		select {
		case events <- Event{Op: Error, Err: err}:
		case <-done:
			return false
		}
	}
	for _, name := range entries {
		log.Trace().Str("name", name).Msg("File listed")
		select {
		case events <- Event{Op: List, Name: name}:
		case <-done:
			return false
		}
	}

	select {
	case events <- Event{Op: List}:
	case <-done:
		return false
	}
	return true
}
