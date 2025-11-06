package backup

import (
	"github.com/PlakarKorp/plakar/appcontext"
)

type eventsProcessorStdio struct {
	done chan struct{}
}

func startEventsProcessorStdio(ctx *appcontext.AppContext, quiet bool) eventsProcessorStdio {
	done := make(chan struct{})
	ep := eventsProcessorStdio{done: done}

	go func() {
		for event := range ctx.Events() {
			switch event.Type {
			case "snapshot.backup.path.error", "snapshot.backup.directory.error", "snapshot.backup.file.error":
				snapshotID := event.Data["snapshot_id"].([]byte)
				pathname := event.Data["path"].(string)
				errorMessage := event.Data["error"].(string)

				ctx.GetLogger().Stderr("%x: KO %s %s: %s", snapshotID[:4], crossMark, pathname, errorMessage)
			case "snapshot.backup.directory.ok", "snapshot.backup.file.ok":
				if !quiet {
					snapshotID := event.Data["snapshot_id"].([]byte)
					pathname := event.Data["path"].(string)
					ctx.GetLogger().Stdout("%x: OK %s %s", snapshotID[:4], checkMark, pathname)
				}
			case "snapshot.backup.done":
				done <- struct{}{}
			default:
				//ctx.GetLogger().Warn("unknown event: %T", event)
			}
		}
	}()

	return ep
}

func (ep eventsProcessorStdio) Close() {
	<-ep.done
}
