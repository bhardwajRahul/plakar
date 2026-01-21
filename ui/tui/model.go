package tui

import (
	"time"

	"github.com/PlakarKorp/kloset/events"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type Event = events.Event
type appMsg Event

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

type eventsClosedMsg struct{}

func waitForDoneEvent(ch <-chan Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return eventsClosedMsg{}
		}
		return appMsg(e)
	}
}

type cancelledMsg struct{ err error }

func waitForCancel(ctx *appcontext.AppContext) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done() // adapt if your AppContext exposes Done() differently
		return cancelledMsg{err: ctx.Err()}
	}
}

type appModel struct {
	appContext *appcontext.AppContext
	events     <-chan Event

	repo *repository.Repository

	dirty   bool
	appName string

	phase      string
	detail     string
	snapshotID string

	startTime time.Time
	forceQuit bool

	// geometry
	width int

	foundSummary        bool
	summaryPathTotal    uint64
	fileCountTotal      uint64
	directoryCountTotal uint64
	symlinkCountTotal   uint64
	xattrCountTotal     uint64
	totalSize           uint64

	// counts (event-driven, no per-path memory)
	countPath      uint64
	countPathOk    uint64
	countPathError uint64

	countDir      uint64
	countDirOk    uint64
	countDirError uint64

	countFile      uint64
	countFileOk    uint64
	countFileError uint64

	countXattr      uint64
	countXattrOk    uint64
	countXattrError uint64

	countSymlink      uint64
	countSymlinkOk    uint64
	countSymlinkError uint64

	countFileSize      int64
	countDirectorySize int64
	countXattrSize     int64

	// timers
	timerResourcesDone    bool
	timerResourcesBegin   time.Time
	timerResourcesElapsed time.Duration

	timerStructureDone    bool
	timerStructureBegin   time.Time
	timerStructureElapsed time.Duration

	// last pathname
	lastPathname string

	// UI
	barPrefix          string
	ressourcesProgress progress.Model
	structureProgress  progress.Model

	// ETA calculation
	lastETAAt      time.Time
	lastResDone    uint64
	lastStructDone uint64
	resRateEMA     float64 // items/sec
	structRateEMA  float64 // items/sec

	spin spinner.Model

	// buffer view
	lastNPaths  int
	lastPaths   []string
	lastNErrors int
	lastErrors  []string
}

func newGenericModel(ctx *appcontext.AppContext, appName string, events <-chan Event, repo *repository.Repository) tea.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Line

	rpr := progressBar()
	spr := progressBar()

	return appModel{
		repo:               repo,
		appName:            appName,
		appContext:         ctx,
		events:             events,
		startTime:          time.Now(),
		ressourcesProgress: rpr,
		structureProgress:  spr,
		spin:               sp,
		lastNPaths:         ctx.MaxConcurrency,
		lastNErrors:        ctx.MaxConcurrency,
		lastPaths:          make([]string, 0, ctx.MaxConcurrency),
		lastErrors:         make([]string, 0, ctx.MaxConcurrency),
	}
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		m.spin.Tick,
		tick(),
		waitForDoneEvent(m.events), // exactly one waiter in flight
		waitForCancel(m.appContext),
	)
}
