package check

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
			case "snapshot.check.directory.missing":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				pathname := event.Data["path"].(string)
				ctx.GetLogger().Warn("%x: %s %s: missing directory", snapshotID[:4], crossMark, pathname)
			case "snapshot.check.file.missing":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				pathname := event.Data["path"].(string)
				ctx.GetLogger().Warn("%x: %s %s: missing file", snapshotID[:4], crossMark, pathname)
			case "snapshot.check.object.missing":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				contentMac := event.Data["content_mac"].([32]byte)
				ctx.GetLogger().Warn("%x: %s %x: missing object", snapshotID[:4], crossMark, contentMac)
			case "snapshot.check.chunk.missing":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				contentMac := event.Data["content_mac"].([32]byte)
				ctx.GetLogger().Warn("%x: %s %x: missing chunk", snapshotID[:4], crossMark, contentMac)

			case "snapshot.check.directory.corrupted":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				pathname := event.Data["path"].(string)
				ctx.GetLogger().Warn("%x: %s %s: corrupted directory", snapshotID[:4], crossMark, pathname)
			case "snapshot.check.file.corrupted":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				pathname := event.Data["path"].(string)
				ctx.GetLogger().Warn("%x: %s %s: corrupted file", snapshotID[:4], crossMark, pathname)
			case "snapshot.check.object.corrupted":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				contentMac := event.Data["content_mac"].([32]byte)
				ctx.GetLogger().Warn("%x: %s %x: corrupted object", snapshotID[:4], crossMark, contentMac)
			case "snapshot.check.chunk.corrupted":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				contentMac := event.Data["content_mac"].([32]byte)
				ctx.GetLogger().Warn("%x: %s %x: corrupted chunk", snapshotID[:4], crossMark, contentMac)

			case "snapshot.check.directory.ok", "snapshot.check.file.ok":
				if !quiet {
					snapshotID := event.Data["snapshot_id"].([16]byte)
					pathname := event.Data["path"].(string)
					ctx.GetLogger().Info("%x: %s %s", snapshotID[:4], checkMark, pathname)
				}

			case "snapshot.check.directory.error", "snapshot.check.file.error":
				snapshotID := event.Data["snapshot_id"].([16]byte)
				pathname := event.Data["path"].(string)
				errorMessage := event.Data["error"].(string)
				ctx.GetLogger().Stderr("%x: KO %s %s: %s", snapshotID[:4], crossMark, pathname, errorMessage)
			default:
			}
		}
		done <- struct{}{}
	}()
	return done
}
