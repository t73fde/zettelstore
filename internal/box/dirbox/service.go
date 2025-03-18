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

package dirbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsx/input"

	"zettelstore.de/z/internal/box/filebox"
	"zettelstore.de/z/internal/box/notify"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/logger"
	"zettelstore.de/z/internal/zettel"
)

func fileService(i uint32, log *logger.Logger, dirPath string, cmds <-chan fileCmd) {
	// Something may panic. Ensure a running service.
	defer func() {
		if ri := recover(); ri != nil {
			kernel.Main.LogRecover("FileService", ri)
			go fileService(i, log, dirPath, cmds)
		}
	}()

	log.Debug().Uint("i", uint64(i)).Str("dirpath", dirPath).Msg("File service started")
	for cmd := range cmds {
		cmd.run(dirPath)
	}
	log.Debug().Uint("i", uint64(i)).Str("dirpath", dirPath).Msg("File service stopped")
}

type fileCmd interface {
	run(string)
}

const serviceTimeout = 5 * time.Second // must be shorter than the web servers timeout values for reading+writing.

// COMMAND: srvGetMeta ----------------------------------------
//
// Retrieves the meta data from a zettel.

func (dp *dirBox) srvGetMeta(ctx context.Context, entry *notify.DirEntry, zid id.Zid) (*meta.Meta, error) {
	rc := make(chan resGetMeta, 1)
	dp.getFileChan(zid) <- &fileGetMeta{entry, rc}
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	select {
	case res := <-rc:
		return res.meta, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type fileGetMeta struct {
	entry *notify.DirEntry
	rc    chan<- resGetMeta
}
type resGetMeta struct {
	meta *meta.Meta
	err  error
}

func (cmd *fileGetMeta) run(dirPath string) {
	var m *meta.Meta
	var err error

	entry := cmd.entry
	zid := entry.Zid
	if metaName := entry.MetaName; metaName == "" {
		contentName := entry.ContentName
		contentExt := entry.ContentExt
		if contentName == "" || contentExt == "" {
			err = fmt.Errorf("no meta, no content in getMeta, zid=%v", zid)
		} else if entry.HasMetaInContent() {
			m, _, err = parseMetaContentFile(zid, filepath.Join(dirPath, contentName))
		} else {
			m = filebox.CalcDefaultMeta(zid, contentExt)
		}
	} else {
		m, err = parseMetaFile(zid, filepath.Join(dirPath, metaName))
	}
	if err == nil {
		cmdCleanupMeta(m, entry)
	}
	cmd.rc <- resGetMeta{m, err}
}

// COMMAND: srvGetMetaContent ----------------------------------------
//
// Retrieves the meta data and the content of a zettel.

func (dp *dirBox) srvGetMetaContent(ctx context.Context, entry *notify.DirEntry, zid id.Zid) (*meta.Meta, []byte, error) {
	rc := make(chan resGetMetaContent, 1)
	dp.getFileChan(zid) <- &fileGetMetaContent{entry, rc}
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	select {
	case res := <-rc:
		return res.meta, res.content, res.err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

type fileGetMetaContent struct {
	entry *notify.DirEntry
	rc    chan<- resGetMetaContent
}
type resGetMetaContent struct {
	meta    *meta.Meta
	content []byte
	err     error
}

func (cmd *fileGetMetaContent) run(dirPath string) {
	var m *meta.Meta
	var content []byte
	var err error

	entry := cmd.entry
	zid := entry.Zid
	contentName := entry.ContentName
	contentExt := entry.ContentExt
	contentPath := filepath.Join(dirPath, contentName)
	if metaName := entry.MetaName; metaName == "" {
		if contentName == "" || contentExt == "" {
			err = fmt.Errorf("no meta, no content in getMetaContent, zid=%v", zid)
		} else if entry.HasMetaInContent() {
			m, content, err = parseMetaContentFile(zid, contentPath)
		} else {
			m = filebox.CalcDefaultMeta(zid, contentExt)
			content, err = os.ReadFile(contentPath)
		}
	} else {
		m, err = parseMetaFile(zid, filepath.Join(dirPath, metaName))
		if contentName != "" {
			var err1 error
			content, err1 = os.ReadFile(contentPath)
			if err == nil {
				err = err1
			}
		}
	}
	if err == nil {
		cmdCleanupMeta(m, entry)
	}
	cmd.rc <- resGetMetaContent{m, content, err}
}

// COMMAND: srvSetZettel ----------------------------------------
//
// Writes a new or exsting zettel.

func (dp *dirBox) srvSetZettel(ctx context.Context, entry *notify.DirEntry, zettel zettel.Zettel) error {
	rc := make(chan resSetZettel, 1)
	dp.getFileChan(zettel.Meta.Zid) <- &fileSetZettel{entry, zettel, rc}
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	select {
	case err := <-rc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type fileSetZettel struct {
	entry  *notify.DirEntry
	zettel zettel.Zettel
	rc     chan<- resSetZettel
}
type resSetZettel = error

func (cmd *fileSetZettel) run(dirPath string) {
	var err error
	entry := cmd.entry
	zid := entry.Zid
	contentName := entry.ContentName
	m := cmd.zettel.Meta
	content := cmd.zettel.Content.AsBytes()
	metaName := entry.MetaName
	if metaName == "" {
		if contentName == "" {
			err = fmt.Errorf("no meta, no content in setZettel, zid=%v", zid)
		} else {
			contentPath := filepath.Join(dirPath, contentName)
			if entry.HasMetaInContent() {
				err = writeZettelFile(contentPath, m, content)
				cmd.rc <- err
				return
			}
			err = writeFileContent(contentPath, content)
		}
		cmd.rc <- err
		return
	}

	err = writeMetaFile(filepath.Join(dirPath, metaName), m)
	if err == nil && contentName != "" {
		err = writeFileContent(filepath.Join(dirPath, contentName), content)
	}
	cmd.rc <- err
}

func writeMetaFile(metaPath string, m *meta.Meta) error {
	metaFile, err := openFileWrite(metaPath)
	if err != nil {
		return err
	}
	err = writeFileZid(metaFile, m.Zid)
	if err == nil {
		_, err = m.WriteComputed(metaFile)
	}
	if err1 := metaFile.Close(); err == nil {
		err = err1
	}
	return err
}

func writeZettelFile(contentPath string, m *meta.Meta, content []byte) error {
	zettelFile, err := openFileWrite(contentPath)
	if err != nil {
		return err
	}
	err = writeMetaHeader(zettelFile, m)
	if err == nil {
		_, err = zettelFile.Write(content)
	}
	if err1 := zettelFile.Close(); err == nil {
		err = err1
	}
	return err
}

var (
	newline = []byte{'\n'}
	yamlSep = []byte{'-', '-', '-', '\n'}
)

func writeMetaHeader(w io.Writer, m *meta.Meta) (err error) {
	if m.YamlSep {
		_, err = w.Write(yamlSep)
		if err != nil {
			return err
		}
	}
	err = writeFileZid(w, m.Zid)
	if err != nil {
		return err
	}
	_, err = m.WriteComputed(w)
	if err != nil {
		return err
	}
	if m.YamlSep {
		_, err = w.Write(yamlSep)
	} else {
		_, err = w.Write(newline)
	}
	return err
}

// COMMAND: srvDeleteZettel ----------------------------------------
//
// Deletes an existing zettel.

func (dp *dirBox) srvDeleteZettel(ctx context.Context, entry *notify.DirEntry, zid id.Zid) error {
	rc := make(chan resDeleteZettel, 1)
	dp.getFileChan(zid) <- &fileDeleteZettel{entry, rc}
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	select {
	case err := <-rc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type fileDeleteZettel struct {
	entry *notify.DirEntry
	rc    chan<- resDeleteZettel
}
type resDeleteZettel = error

func (cmd *fileDeleteZettel) run(dirPath string) {
	var err error

	entry := cmd.entry
	contentName := entry.ContentName
	contentPath := filepath.Join(dirPath, contentName)
	if metaName := entry.MetaName; metaName == "" {
		if contentName == "" {
			err = fmt.Errorf("no meta, no content in deleteZettel, zid=%v", entry.Zid)
		} else {
			err = os.Remove(contentPath)
		}
	} else {
		if contentName != "" {
			err = os.Remove(contentPath)
		}
		err1 := os.Remove(filepath.Join(dirPath, metaName))
		if err == nil {
			err = err1
		}
	}
	for _, dupName := range entry.UselessFiles {
		err1 := os.Remove(filepath.Join(dirPath, dupName))
		if err == nil {
			err = err1
		}
	}
	cmd.rc <- err
}

// Utility functions ----------------------------------------

func parseMetaFile(zid id.Zid, path string) (*meta.Meta, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	inp := input.NewInput(src)
	return meta.NewFromInput(zid, inp), nil
}

func parseMetaContentFile(zid id.Zid, path string) (*meta.Meta, []byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	inp := input.NewInput(src)
	meta := meta.NewFromInput(zid, inp)
	return meta, src[inp.Pos:], nil
}

func cmdCleanupMeta(m *meta.Meta, entry *notify.DirEntry) {
	filebox.CleanupMeta(
		m,
		entry.Zid,
		entry.ContentExt,
		entry.MetaName != "",
		entry.UselessFiles,
	)
}

// fileMode to create a new file: user, group, and all are allowed to read and write.
//
// If you want to forbid others or the group to read or to write, you must set
// umask(1) accordingly.
const fileMode os.FileMode = 0666 //

func openFileWrite(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
}

func writeFileZid(w io.Writer, zid id.Zid) error {
	_, err := io.WriteString(w, "id: ")
	if err == nil {
		_, err = w.Write(zid.Bytes())
		if err == nil {
			_, err = io.WriteString(w, "\n")
		}
	}
	return err
}

func writeFileContent(path string, content []byte) error {
	f, err := openFileWrite(path)
	if err == nil {
		_, err = f.Write(content)
		if err1 := f.Close(); err == nil {
			err = err1
		}
	}
	return err
}
