package tui

import (
	"errors"

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
	err    error
}

var apps = map[string]func(*appcontext.AppContext, string, <-chan Event, *repository.Repository) tea.Model{
	"backup": newGenericModel,
	"export": newGenericModel,
	//"check":  newGenericModel,
}

type tui struct {
	ctx  *appcontext.AppContext
	repo *repository.Repository
	done chan error
}

func New(ctx *appcontext.AppContext) ui.UI {
	return &tui{
		ctx: ctx,
	}
}

func (tui *tui) Wait() error {
	return <-tui.done
}

func (tui *tui) SetRepository(repo *repository.Repository) {
	tui.repo = repo
}

func (tui *tui) Run() error {
	events := tui.ctx.Events().Listen()
	tui.done = make(chan error, 1)

	go func() {
		defer close(tui.done)

		var app *commandApp

		for e := range events {
			// If app is running, forward event (non-blocking / cancel-safe)
			if app != nil {
				select {
				case app.events <- *e:
				case <-app.done:
					if app.err != nil {
						if errors.Is(app.err, tea.ErrInterrupted) {
							tui.done <- ui.ErrUserAbort
						} else {
							tui.done <- app.err
						}
					}
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
						continue
					}
					continue
				}
			}

			// default fallback
			stdio.HandleEvent(tui.ctx, e)
		}
		if app != nil {
			stopApp(app)
			tui.done <- nil
			return
		}
	}()

	return nil
}

func startApp(ctx *appcontext.AppContext, app string, repo *repository.Repository) *commandApp {
	events := make(chan Event, 256)
	done := make(chan struct{})

	modelFunc, ok := apps[app]
	if !ok {
		return nil
	}

	capp := &commandApp{
		events: events,
		done:   done,
		prog:   tea.NewProgram(modelFunc(ctx, app, events, repo)),
	}

	go func() {
		defer close(done)
		_, err := capp.prog.Run()
		if err != nil {
			capp.err = err
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
