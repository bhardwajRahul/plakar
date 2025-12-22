package tui

import (
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/ui/stdio"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type commandApp struct {
	job    uuid.UUID
	events chan Event    // events we feed into the Bubbletea model
	done   chan struct{} // closed when Bubbletea program exits
}

var apps = map[string]func(*appcontext.AppContext, <-chan Event) tea.Model{
	"backup":  newBackupModel,
	"restore": newRestoreModel,
	"check":   newCheckModel,
}

func Run(ctx *appcontext.AppContext) func() {
	events := ctx.Events().Listen()
	done := make(chan error, 1)

	go func() {
		defer close(done)

		var app *commandApp

		for e := range events {

			if app != nil {
				app.events <- *e
				if e.Type == "workflow.end" && e.Job == app.job {
					close(app.events)
					<-app.done
					app = nil
				}
				continue
			}

			if e.Type == "workflow.start" {
				app = startApp(ctx, e.Data["workflow"].(string))
				if app != nil {
					app.job = e.Job
					app.events <- *e
					continue
				}
			}

			stdio.HandleEvent(ctx, e)
		}

		if app != nil {
			close(app.events)
			<-app.done
			app = nil
		}

		done <- nil
	}()

	// wait function, as before
	return func() {
		<-done
	}
}

func startApp(ctx *appcontext.AppContext, app string) *commandApp {
	events := make(chan Event, 256) // larger buffer to avoid stalls without dropping
	done := make(chan struct{})

	if modelFunc, ok := apps[app]; !ok {
		return nil
	} else {
		m := modelFunc(ctx, events)
		p := tea.NewProgram(m) // optionally: tea.WithOutput(ctx.Stdout)

		go func() {
			defer close(done)
			if _, err := p.Run(); err != nil {
				ctx.GetLogger().Stderr("backup TUI error: %v", err)
			}
		}()
	}

	return &commandApp{
		events: events,
		done:   done,
	}
}
