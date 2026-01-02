//go:build linux || darwin

package plakarfs

import (
	"io/fs"
	"time"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	fusefs "github.com/anacrolix/fuse/fs"
)

type plakarFS struct {
	ctx *appcontext.AppContext

	repo          *repository.Repository
	locateOptions *locate.LocateOptions
	chrootfs      fs.FS

	rootRefresh    time.Duration
	kernelCacheTTL time.Duration
	inodeCache     *inodeCache
}

func NewFS(ctx *appcontext.AppContext, repo *repository.Repository, locateOptions *locate.LocateOptions, chrootfs fs.FS) *plakarFS {
	return &plakarFS{
		ctx:            ctx,
		repo:           repo,
		locateOptions:  locateOptions,
		chrootfs:       chrootfs,
		rootRefresh:    10 * time.Second,
		kernelCacheTTL: time.Minute,
		inodeCache:     newInodeCache(),
	}
}

func (fs *plakarFS) Root() (fusefs.Node, error) {
	return NewDirectory(fs, fs.chrootfs, nil, "")
}
