package tui

import (
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/ui"
	"github.com/PlakarKorp/plakar/ui/stdio"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type commandApp struct {
	job    uuid.UUID
	events chan Event    // events we feed into the Bubbletea model
	done   chan struct{} // closed when Bubbletea program exits
	prog   *tea.Program
}

var apps = map[string]func(*appcontext.AppContext, string, <-chan Event, *repository.Repository) tea.Model{
	"backup":  newGenericModel,
	"restore": newGenericModel,
	"check":   newGenericModel,
}

type tui struct {
	ctx  *appcontext.AppContext
	repo *repository.Repository
}

func New(ctx *appcontext.AppContext) ui.UI {
	return &tui{
		ctx: ctx,
	}
}

func (tui *tui) SetRepository(repo *repository.Repository) {
	tui.repo = repo
}

func (tui *tui) Run() func() {
	events := tui.ctx.Events().Listen()
	done := make(chan error, 1)

	go func() {
		defer close(done)

		var app *commandApp

		for {
			select {
			case <-tui.ctx.Done():
				stopApp(app)
				done <- tui.ctx.Err()
				return

			case e, ok := <-events:
				if !ok {
					stopApp(app)
					done <- nil
					return
				}

				// If app is running, forward event (non-blocking / cancel-safe)
				if app != nil {
					select {
					case app.events <- *e:
					case <-app.done:
						app = nil
					case <-tui.ctx.Done():
						stopApp(app)
						done <- tui.ctx.Err()
						return
					}

					// Close app on matching workflow.end
					if e.Type == "workflow.end" && e.Job == app.job {
						stopApp(app)
						app = nil
					}
					continue
				}

				// No app: start when workflow.start matches a known model
				if e.Type == "workflow.start" {
					app = startApp(tui.ctx, e.Data["workflow"].(string), tui.repo)
					if app != nil {
						app.job = e.Job
						// feed start event (also cancel-safe)
						select {
						case app.events <- *e:
						case <-tui.ctx.Done():
							stopApp(app)
							done <- tui.ctx.Err()
							return
						}
						continue
					}
				}

				// default fallback
				stdio.HandleEvent(tui.ctx, e)
			}
		}
	}()

	return func() { <-done }
}

func startApp(ctx *appcontext.AppContext, app string, repo *repository.Repository) *commandApp {
	events := make(chan Event, 256)
	done := make(chan struct{})

	modelFunc, ok := apps[app]
	if !ok {
		return nil
	}

	m := modelFunc(ctx, app, events, repo)
	p := tea.NewProgram(m)

	capp := &commandApp{
		events: events,
		done:   done,
		prog:   p,
	}

	go func() {
		defer close(done)
		if _, err := p.Run(); err != nil {
			ctx.GetLogger().Stderr("tui error: %v", err)
		}
	}()

	return capp
}

func stopApp(app *commandApp) {
	if app == nil {
		return
	}
	if app.prog != nil {
		app.prog.Quit()
	}

	close(app.events)
	<-app.done
}
