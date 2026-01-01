//go:build linux || darwin

package plakarfs

import (
	"context"
	"io"
	"syscall"

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
	fs  *FS
	vfs *vfs.Filesystem

	path string

	cacheKey string
	attr     *fuse.Attr
}

func NewFile(fs *FS, vfs *vfs.Filesystem, parent *Dir, path string) (*File, error) {
	key := stableKey("file", parent.snapKey, path)
	if f, ok := fs.inodeCache.getFile(key); ok {
		return f, nil
	} else {
		entry, err := vfs.GetEntryNoFollow(path)
		if err != nil {
			return nil, syscall.ENOENT
		}
		f := &File{
			fs:       fs,
			vfs:      vfs,
			path:     path,
			cacheKey: key,
			attr: &fuse.Attr{
				Valid: fs.kernelCacheTTL,
				Mode:  entry.Stat().Mode(),
				Uid:   uint32(entry.Stat().Uid()),
				Gid:   uint32(entry.Stat().Gid()),
				Ctime: entry.Stat().ModTime(),
				Mtime: entry.Stat().ModTime(),
				Atime: entry.Stat().ModTime(),
				Size:  uint64(entry.Stat().Size()),
				Nlink: uint32(entry.Stat().Nlink()),
			},
		}
		fs.inodeCache.setFile(f.cacheKey, f)
		return f, nil
	}
}

func (f *File) Forget() { f.fs.inodeCache.removeFile(f.cacheKey) }

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	*a = *f.attr
	if a.Mode.IsDir() {
		return syscall.EISDIR
	}
	return nil
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	resp.Flags |= fuse.OpenDirectIO
	resp.Flags |= fuse.OpenKeepCache

	rd, err := f.vfs.Open(f.path)
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
