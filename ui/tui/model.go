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

type cancelledMsg struct{ err error }

func waitForCancel(ctx *appcontext.AppContext) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done() // adapt if your AppContext exposes Done() differently
		return cancelledMsg{err: ctx.Err()}
	}
}

type appBatchMsg struct {
	events []Event
}

func waitForBatch(ch <-chan Event, max int) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return eventsClosedMsg{}
		}
		batch := make([]Event, 0, max)
		batch = append(batch, e)

		// drain without blocking
		for len(batch) < max {
			select {
			case e2, ok := <-ch:
				if !ok {
					return eventsClosedMsg{}
				}
				batch = append(batch, e2)
			default:
				return appBatchMsg{events: batch}
			}
		}
		return appBatchMsg{events: batch}
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
	width  int
	height int

	foundSummary        bool
	summaryPathTotal    uint64
	fileCountTotal      uint64
	directoryCountTotal uint64
	symlinkCountTotal   uint64
	xattrCountTotal     uint64
	totalSize           uint64

	// counts (event-driven, no per-path memory)
	countPath       uint64
	countPathOk     uint64
	countPathError  uint64
	countPathCached uint64

	countDir       uint64
	countDirOk     uint64
	countDirError  uint64
	countDirCached uint64

	countFile       uint64
	countFileOk     uint64
	countFileError  uint64
	countFileCached uint64

	countXattr       uint64
	countXattrOk     uint64
	countXattrError  uint64
	countXattrCached uint64

	countSymlink       uint64
	countSymlinkOk     uint64
	countSymlinkError  uint64
	countSymlinkCached uint64

	countFileSize      int64
	countDirectorySize int64
	countXattrSize     int64
	countCachedSize    uint64

	// timers
	timerResourcesDone    bool
	timerResourcesBegin   time.Time
	timerResourcesElapsed time.Duration

	timerStructureDone    bool
	timerStructureBegin   time.Time
	timerStructureElapsed time.Duration

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
	lastItem string
	errors   []string
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
		lastItem:           "",
		errors:             []string{},
	}
}

func (m appModel) Init() tea.Cmd {
	const batchMax = 1024 // tune: 128/256/512/1024 depending on workload

	return tea.Batch(
		m.spin.Tick,
		tick(),
		waitForBatch(m.events, batchMax), // exactly one batch waiter in flight
		waitForCancel(m.appContext),
	)
}
