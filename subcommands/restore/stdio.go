package restore

import (
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/charmbracelet/lipgloss"
)

var (
	checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).SetString("✓")
	crossMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).SetString("✘")
)

func eventsProcessorStdio(ctx *appcontext.AppContext, quiet bool) chan struct{} {
	done := make(chan struct{})
	go func() {
		for event := range ctx.Events().Listen() {
			switch event.Type {
			case "snapshot.restore.path.error", "snapshot.restore.directory.error", "snapshot.restore.file.error":
				snapshotID := event.Data["snapshot_id"].([]byte)
				pathname := event.Data["path"].(string)
				errorMessage := event.Data["error"].(string)
				ctx.GetLogger().Warn("%x: KO %s %s: %s", snapshotID[:4], crossMark, pathname, errorMessage)

			case "snapshot.restore.path.ok", "snapshot.restore.directory.ok", "snapshot.restore.file.ok":
				if !quiet {
					snapshotID := event.Data["snapshot_id"].([]byte)
					pathname := event.Data["path"].(string)
					ctx.GetLogger().Info("%x: OK %s %s", snapshotID[:4], checkMark, pathname)
				}
			default:
			}
		}
		done <- struct{}{}
	}()
	return done
}
