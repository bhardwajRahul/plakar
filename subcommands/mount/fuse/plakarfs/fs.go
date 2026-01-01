//go:build linux || darwin

package plakarfs

import (
	"time"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/anacrolix/fuse/fs"
)

type FS struct {
	ctx *appcontext.AppContext

	repo          *repository.Repository
	locateOptions *locate.LocateOptions

	kernelCacheTTL time.Duration
	inodeCache     *inodeCache
}

func NewFS(ctx *appcontext.AppContext, repo *repository.Repository, locateOptions *locate.LocateOptions) *FS {
	return &FS{
		ctx:            ctx,
		repo:           repo,
		locateOptions:  locateOptions,
		kernelCacheTTL: time.Minute,
		inodeCache:     newInodeCache(),
	}
}

func (fs *FS) Root() (fs.Node, error) {
	return NewDirectory(fs, nil, nil, "/")
}
