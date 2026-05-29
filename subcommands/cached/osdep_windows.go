package cached

import (
	"errors"
	"io"

	"github.com/PlakarKorp/plakar/appcontext"
)

func setupSyslog(ctx *appcontext.AppContext) error {
	ctx.GetLogger().SetOutput(io.Discard)
	return nil
}

func daemonize(argv []string) error {
	return errors.ErrUnsupported
}
