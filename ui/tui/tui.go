package tui

import (
	"errors"
	"io"
	"os"

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
	"import": newGenericModel,
	"export": newGenericModel,
}

type tui struct {
	ctx  *appcontext.AppContext
	repo *repository.Repository

	app *commandApp

	done chan error
}

func New(ctx *appcontext.AppContext) ui.UI {
	return &tui{
		ctx: ctx,
	}
}

func (t *tui) Stdout() io.Writer {
	return &switchWriter{
		tui:      t,
		stream:   "stdout",
		fallback: os.Stdout,
	}
}

func (t *tui) Stderr() io.Writer {
	return &switchWriter{
		tui:      t,
		stream:   "stderr",
		fallback: os.Stderr,
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

		hasSeenStop := false
		for e := range events {
			// If app is running, forward event (non-blocking / cancel-safe)
			if tui.app != nil {
				select {
				case tui.app.events <- *e:
				case <-tui.app.done:
					if tui.app.err != nil {
						if errors.Is(tui.app.err, tea.ErrInterrupted) {
							if !hasSeenStop {
								hasSeenStop = true
								tui.done <- ui.ErrUserAbort
							}
						} else {
							if !hasSeenStop {
								hasSeenStop = true
								tui.done <- tui.app.err
							}
						}
					}
				}

				// Close app on matching workflow.end
				if e.Type == "workflow.end" && e.Job == tui.app.job {
					stopApp(tui.app)
					tui.app = nil
				}
				continue
			}

			// No app: start when workflow.start matches a known model
			if e.Type == "workflow.start" {
				tui.app = startApp(tui.ctx, e.Data["workflow"].(string), tui.repo)
				if tui.app != nil {
					tui.app.job = e.Job
					// feed start event (also cancel-safe)
					select {
					case tui.app.events <- *e:
					case <-tui.ctx.Done():
						continue
					}
					continue
				}
			}

			// default fallback
			stdio.HandleEvent(tui.ctx, e)
		}
		if tui.app != nil {
			stopApp(tui.app)
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
