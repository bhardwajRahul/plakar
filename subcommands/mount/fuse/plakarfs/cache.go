//go:build linux || darwin

package plakarfs

import (
	"strings"
	"sync"
)

type inodeCache struct {
	muFiles sync.Mutex
	files   map[string]*File

	muDirs sync.Mutex
	dirs   map[string]*Dir
}

func newInodeCache() *inodeCache {
	return &inodeCache{
		files: make(map[string]*File),
		dirs:  make(map[string]*Dir),
	}
}

func stableKey(parts ...string) string {
	return strings.Join(append([]string{}, parts...), "/")
}

func (ic *inodeCache) setFile(key string, file *File) {
	ic.muFiles.Lock()
	ic.files[key] = file
	ic.muFiles.Unlock()
}

func (ic *inodeCache) getFile(key string) (*File, bool) {
	ic.muFiles.Lock()
	v, ok := ic.files[key]
	ic.muFiles.Unlock()
	return v, ok
}

func (ic *inodeCache) removeFile(key string) {
	ic.muFiles.Lock()
	delete(ic.files, key)
	ic.muFiles.Unlock()
}

func (ic *inodeCache) setDir(key string, dir *Dir) {
	ic.muDirs.Lock()
	ic.dirs[key] = dir
	ic.muDirs.Unlock()
}

func (ic *inodeCache) getDir(key string) (*Dir, bool) {
	ic.muDirs.Lock()
	v, ok := ic.dirs[key]
	ic.muDirs.Unlock()
	return v, ok
}

func (ic *inodeCache) removeDir(key string) {
	ic.muDirs.Lock()
	delete(ic.dirs, key)
	ic.muDirs.Unlock()
}
