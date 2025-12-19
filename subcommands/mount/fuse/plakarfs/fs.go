//go:build linux || darwin

package plakarfs

import (
	"sync"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/anacrolix/fuse/fs"
)

func stableIno(parts ...string) uint64 {
	const off = 1469598103934665603
	const prime = 1099511628211
	h := uint64(off)
	for _, s := range parts {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= prime
		}
	}
	if h < 2 {
		h += 2
	}
	return h
}

type FS struct {
	ctx *appcontext.AppContext

	repo          *repository.Repository
	locateOptions *locate.LocateOptions

	muFiles sync.Mutex
	files   map[uint64]*File

	muDirs sync.Mutex
	dirs   map[uint64]*Dir
}

func NewFS(ctx *appcontext.AppContext, repo *repository.Repository, locateOptions *locate.LocateOptions) *FS {
	return &FS{
		ctx:           ctx,
		repo:          repo,
		locateOptions: locateOptions,
		files:         make(map[uint64]*File),
		dirs:          make(map[uint64]*Dir),
	}
}

func (f *FS) CacheFile(file *File) {
	f.muFiles.Lock()
	f.files[file.ino] = file
	f.muFiles.Unlock()
}

func (f *FS) GetFile(ino uint64) (*File, bool) {
	f.muFiles.Lock()
	v, ok := f.files[ino]
	f.muFiles.Unlock()
	return v, ok
}

func (f *FS) RemoveFile(ino uint64) {
	f.muFiles.Lock()
	delete(f.files, ino)
	f.muFiles.Unlock()
}

func (f *FS) CacheDir(dir *Dir) {
	f.muDirs.Lock()
	f.dirs[dir.ino] = dir
	f.muDirs.Unlock()
}

func (f *FS) GetDir(ino uint64) (*Dir, bool) {
	f.muDirs.Lock()
	v, ok := f.dirs[ino]
	f.muDirs.Unlock()
	return v, ok
}

func (f *FS) RemoveDir(ino uint64) {
	f.muDirs.Lock()
	delete(f.dirs, ino)
	f.muDirs.Unlock()
}

func (f *FS) Root() (fs.Node, error) {
	const rootIno = uint64(1)

	if root, ok := f.GetDir(rootIno); ok {
		return root, nil
	}

	root := &Dir{
		fs:       f,
		repo:     f.repo,
		name:     "/",
		fullpath: "/",
		ino:      rootIno,
	}
	f.CacheDir(root)
	return root, nil
}
