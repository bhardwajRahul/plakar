/*
 * Copyright (c) 2023 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package fs

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/PlakarKorp/kloset/location"
	"github.com/PlakarKorp/kloset/objects"
	"github.com/PlakarKorp/kloset/snapshot/exporter"
)

type FSExporter struct {
	rootDir string
}

func init() {
	exporter.Register("fs", location.FLAG_LOCALFS, NewFSExporter)
}

func NewFSExporter(ctx context.Context, opts *exporter.Options, name string, config map[string]string) (exporter.Exporter, error) {
	return &FSExporter{
		rootDir: strings.TrimPrefix(config["location"], "fs://"),
	}, nil
}

func (p *FSExporter) Root(ctx context.Context) (string, error) {
	return p.rootDir, nil
}

func (p *FSExporter) CreateDirectory(ctx context.Context, pathname string) error {
	return os.MkdirAll(pathname, 0700)
}

func (p *FSExporter) StoreFile(ctx context.Context, pathname string, fp io.Reader, size int64) error {
	f, err := os.Create(pathname)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, fp); err != nil {
		//logging.Warn("copy failure: %s: %s", pathname, err)
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		//logging.Warn("sync failure: %s: %s", pathname, err)
	}
	if err := f.Close(); err != nil {
		//logging.Warn("close failure: %s: %s", pathname, err)
	}
	return nil
}

func (p *FSExporter) SetPermissions(ctx context.Context, pathname string, fileinfo *objects.FileInfo) error {
	if err := os.Chmod(pathname, fileinfo.Mode()); err != nil {
		return err
	}
	if os.Getuid() == 0 {
		if err := os.Chown(pathname, int(fileinfo.Uid()), int(fileinfo.Gid())); err != nil {
			return err
		}
	}
	if err := os.Chtimes(pathname, fileinfo.ModTime(), fileinfo.ModTime()); err != nil {
		return err
	}
	return nil
}

func (p *FSExporter) CreateLink(ctx context.Context, oldname string, newname string, ltype exporter.LinkType) error {
	if ltype == exporter.HARDLINK {
		return os.Link(oldname, newname)
	}

	return os.Symlink(oldname, newname)
}

func (p *FSExporter) Close(ctx context.Context) error {
	return nil
}
