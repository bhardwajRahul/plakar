//go:build linux || darwin

package plakarfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/kloset/snapshot/vfs"
	"github.com/PlakarKorp/plakar/cached"
	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
)

type Dir struct {
	fs       *FS
	parent   *Dir
	name     string
	fullpath string

	repo *repository.Repository
	snap *snapshot.Snapshot
	vfs  *vfs.Filesystem

	ino uint64
}

func (d *Dir) Forget() { d.fs.RemoveDir(d.ino) }

func (d *Dir) snapKey() string {
	p := d
	for p.parent != nil && p.parent.name != "/" {
		p = p.parent
	}
	if p.parent != nil && p.parent.name == "/" {
		return p.name
	}
	return "" // root dir
}

func (d *Dir) ensureInit() error {
	if d.name == "/" {
		d.fullpath = "/"
		return nil
	}

	if d.parent != nil && d.parent.name == "/" {
		if d.vfs != nil && d.snap != nil && d.repo != nil {
			return nil
		}

		snap, _, err := locate.OpenSnapshotByPath(d.repo, d.name)
		if err != nil {
			return syscall.ENOENT
		}
		snapfs, err := snap.Filesystem()
		if err != nil {
			return err
		}

		d.snap = snap
		d.repo = d.parent.repo
		d.vfs = snapfs

		// snapshot root in VFS
		d.fullpath = "/"
		return nil
	}

	if d.parent != nil {
		if err := d.parent.ensureInit(); err != nil {
			return err
		}
		d.snap = d.parent.snap
		d.repo = d.parent.repo
		d.vfs = d.parent.vfs

		// fullpath should be set at creation, safety net
		if d.fullpath == "" {
			d.fullpath = filepath.Clean(d.parent.fullpath + "/" + d.name)
		}
		return nil
	}

	return nil
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	if err := d.ensureInit(); err != nil {
		return err
	}

	a.Valid = time.Minute
	a.Inode = d.ino
	a.Uid = uint32(os.Geteuid())
	a.Gid = uint32(os.Getgid())
	a.Nlink = 2

	// root dir is virtual
	if d.name == "/" {
		a.Mode = os.ModeDir | 0o700
		return nil
	}

	// snapshot dir is virtual too
	if d.parent != nil && d.parent.name == "/" {
		a.Mode = os.ModeDir | 0o700
		ts := d.snap.Header.Timestamp
		a.Ctime, a.Mtime, a.Atime = ts, ts, ts
		a.Size = d.snap.Header.GetSource(0).Summary.Directory.Size + d.snap.Header.GetSource(0).Summary.Below.Size
		return nil
	}

	// real directory inside snapshot
	fi, err := d.vfs.GetEntry(d.fullpath)
	if err != nil {
		return syscall.ENOENT
	}
	if !fi.Stat().IsDir() {
		return syscall.ENOTDIR
	}

	a.Mode = fi.Stat().Mode()
	a.Uid = uint32(fi.Stat().Uid())
	a.Gid = uint32(fi.Stat().Gid())
	a.Ctime = fi.Stat().ModTime()
	a.Mtime = fi.Stat().ModTime()
	a.Atime = fi.Stat().ModTime()
	a.Size = uint64(fi.Stat().Size())
	return nil
}

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if d.name == "/" {
		ino := stableIno("snapdir", name)
		if child, ok := d.fs.GetDir(ino); ok {
			return child, nil
		}

		child := &Dir{
			fs:       d.fs,
			parent:   d,
			name:     name,
			fullpath: "/", // snapshot VFS root
			repo:     d.repo,
			ino:      ino,
		}
		d.fs.CacheDir(child)
		return child, nil
	}

	if err := d.ensureInit(); err != nil {
		return nil, err
	}

	cleanpath := filepath.Clean(d.fullpath + "/" + name)
	entry, err := d.vfs.GetEntry(cleanpath)
	if err != nil {
		return nil, syscall.ENOENT
	}

	sk := d.snapKey()

	if entry.Stat().IsDir() {
		ino := stableIno("dir", sk, cleanpath)
		if dir, ok := d.fs.GetDir(ino); ok {
			return dir, nil
		}
		dir := &Dir{
			fs:       d.fs,
			parent:   d,
			name:     name,
			fullpath: cleanpath,
			repo:     d.repo,
			snap:     d.snap,
			vfs:      d.vfs,
			ino:      ino,
		}
		d.fs.CacheDir(dir)
		return dir, nil
	}

	ino := stableIno("file", sk, cleanpath)
	if f, ok := d.fs.GetFile(ino); ok {
		return f, nil
	}
	f := &File{
		fs:       d.fs,
		parent:   d,
		name:     name,
		fullpath: cleanpath,
		repo:     d.repo,
		snap:     d.snap,
		vfs:      d.vfs,
		ino:      ino,
	}
	d.fs.CacheFile(f)
	return f, nil
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	if d.name == "/" {
		_, err := cached.RebuildStateFromCached(d.fs.ctx, d.repo.Configuration().RepositoryID, d.fs.ctx.StoreConfig)
		if err != nil {
			return nil, err
		}

		snapshotIDs, err := locate.LocateSnapshotIDs(d.repo, d.fs.locateOptions)
		if err != nil {
			return nil, err
		}

		out := make([]fuse.Dirent, 0, len(snapshotIDs))
		for _, snapshotID := range snapshotIDs {
			idHex := fmt.Sprintf("%x", snapshotID)
			out = append(out, fuse.Dirent{
				Name: idHex[:8],
				Type: fuse.DT_Dir,
			})
		}
		return out, nil
	}

	if err := d.ensureInit(); err != nil {
		return nil, err
	}

	children, err := d.vfs.Children(d.fullpath)
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
