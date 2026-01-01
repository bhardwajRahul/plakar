//go:build linux || darwin

package plakarfs

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/kloset/snapshot/vfs"
	"github.com/PlakarKorp/plakar/cached"
	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
)

type Dir struct {
	fs     *FS
	vfs    *vfs.Filesystem
	parent *Dir

	snap    *snapshot.Snapshot
	snapKey string

	path string

	cacheKey string
	attr     *fuse.Attr
}

func NewDirectory(fs *FS, vfs *vfs.Filesystem, parent *Dir, pathname string) (*Dir, error) {
	var key string
	if vfs == nil && parent == nil {
		key = stableKey(pathname)
	} else if parent == parent.parent {
		key = stableKey("snapdir", pathname)
	} else {
		key = stableKey("dir", parent.snapKey, pathname)
	}

	if child, ok := fs.inodeCache.getDir(key); ok {
		return child, nil
	} else {
		dir := &Dir{
			fs:       fs,
			vfs:      vfs,
			parent:   parent,
			path:     pathname,
			cacheKey: key,
			attr: &fuse.Attr{
				Valid: fs.kernelCacheTTL,
				Uid:   uint32(os.Geteuid()),
				Gid:   uint32(os.Getgid()),
				Nlink: 2,
				Mode:  os.ModeDir | 0o700,
			},
		}
		if parent == nil {
			dir.parent = dir
		}
		if parent != nil {
			dir.snapKey = parent.snapKey
		}
		if !dir.IsRoot() {
			if dir.vfs == nil {
				snap, _, err := locate.OpenSnapshotByPath(fs.repo, strings.TrimPrefix(pathname, "/"))
				if err != nil {
					return nil, syscall.ENOENT
				}
				snapfs, err := snap.Filesystem()
				if err != nil {
					return nil, err
				}
				dir.snap = snap
				dir.vfs = snapfs
				dir.path = "/"
				dir.snapKey = fmt.Sprintf("%x", dir.snap.Header.Identifier[:4])

				dir.attr.Mode = os.ModeDir | 0o700
				ts := snap.Header.Timestamp
				dir.attr.Ctime, dir.attr.Mtime, dir.attr.Atime = ts, ts, ts
				dir.attr.Size = snap.Header.GetSource(0).Summary.Directory.Size + snap.Header.GetSource(0).Summary.Below.Size
			} else {
				entry, err := vfs.GetEntryNoFollow(pathname)
				if err != nil {
					return nil, syscall.ENOENT
				}
				dir.attr.Mode = entry.Stat().Mode()
				dir.attr.Uid = uint32(entry.Stat().Uid())
				dir.attr.Gid = uint32(entry.Stat().Gid())
				dir.attr.Ctime = entry.Stat().ModTime()
				dir.attr.Mtime = entry.Stat().ModTime()
				dir.attr.Atime = entry.Stat().ModTime()
				dir.attr.Size = uint64(entry.Stat().Size())
			}
		}

		fs.inodeCache.setDir(dir.cacheKey, dir)
		return dir, nil
	}
}

func (d *Dir) IsRoot() bool {
	return d.vfs == nil && d.parent == d
}

func (d *Dir) Forget() { d.fs.inodeCache.removeDir(d.cacheKey) }

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	*a = *d.attr
	if !a.Mode.IsDir() {
		return syscall.ENOTDIR
	}
	return nil
}

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if d.IsRoot() {
		// special case, we're at the root
		return NewDirectory(d.fs, nil, d, path.Join("/", name))
	}

	entry, err := d.vfs.GetEntryNoFollow(filepath.Clean(filepath.Join(d.path, name)))
	if err != nil {
		return nil, syscall.ENOENT
	}

	if entry.Stat().IsDir() {
		return NewDirectory(d.fs, d.vfs, d, entry.Path())
	} else {
		return NewFile(d.fs, d.vfs, d, entry.Path())
	}
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	if d.IsRoot() {
		_, err := cached.RebuildStateFromStore(d.fs.ctx, d.fs.repo.Configuration().RepositoryID, d.fs.ctx.StoreConfig)
		if err != nil {
			return nil, err
		}

		snapshotIDs, err := locate.LocateSnapshotIDs(d.fs.repo, d.fs.locateOptions)
		if err != nil {
			return nil, err
		}

		out := make([]fuse.Dirent, 0, len(snapshotIDs))
		for _, snapshotID := range snapshotIDs {
			out = append(out, fuse.Dirent{
				Name: fmt.Sprintf("%x", snapshotID)[:8],
				Type: fuse.DT_Dir,
			})
		}

		return out, nil
	}

	children, err := d.vfs.Children(d.path)
	if err != nil {
		return nil, err
	}

	out := make([]fuse.Dirent, 0)
	for entry, err := range children {
		if err != nil {
			return nil, err
		}
		de := fuse.Dirent{Name: entry.Name(), Type: fuse.DT_File}
		if entry.Stat().IsDir() {
			de.Type = fuse.DT_Dir
		}
		out = append(out, de)
	}
	return out, nil
}
