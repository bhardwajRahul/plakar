//go:build linux || darwin

package plakarfs

import (
	"context"
	"io"
	"syscall"
	"time"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/kloset/snapshot/vfs"
	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
)

var _ fs.Node = (*File)(nil)
var _ fs.NodeOpener = (*File)(nil)

type fileHandle struct {
	f io.ReadCloser
}

var _ fs.Handle = (*fileHandle)(nil)
var _ fs.HandleReader = (*fileHandle)(nil)

type File struct {
	fs     *FS
	parent *Dir
	name   string

	fullpath string
	repo     *repository.Repository
	snap     *snapshot.Snapshot
	vfs      *vfs.Filesystem

	ino uint64
}

func (f *File) Forget() { f.fs.RemoveFile(f.ino) }

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	entry, err := f.vfs.GetEntry(f.fullpath)
	if err != nil {
		return syscall.ENOENT
	}
	if entry.Stat().IsDir() {
		return syscall.EISDIR
	}

	a.Valid = time.Minute
	a.Inode = f.ino
	a.Mode = entry.Stat().Mode()
	a.Uid = uint32(entry.Stat().Uid())
	a.Gid = uint32(entry.Stat().Gid())
	a.Ctime = entry.Stat().ModTime()
	a.Mtime = entry.Stat().ModTime()
	a.Atime = entry.Stat().ModTime()
	a.Size = uint64(entry.Stat().Size())
	a.Nlink = uint32(entry.Stat().Nlink())
	return nil
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	resp.Flags |= fuse.OpenDirectIO
	resp.Flags |= fuse.OpenKeepCache

	rd, err := f.snap.NewReader(f.fullpath)
	if err != nil {
		return nil, err
	}
	return &fileHandle{f: rd}, nil
}

func (h *fileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	ra, ok := h.f.(io.ReaderAt)
	if !ok {
		return syscall.EIO
	}
	buf := make([]byte, req.Size)
	n, err := ra.ReadAt(buf, req.Offset)
	if err != nil && err != io.EOF {
		return err
	}
	resp.Data = buf[:n]
	return nil
}

func (h *fileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return h.f.Close()
}

func (h *fileHandle) Access(ctx context.Context, req *fuse.AccessRequest) error {
	return nil
}
