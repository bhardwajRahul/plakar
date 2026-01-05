package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
)

type backupModel struct {
	appContext *appcontext.AppContext
	events     <-chan Event

	phase      string
	detail     string
	snapshotID string

	startTime time.Time
	forceQuit bool

	foundSummary        bool
	fileCountTotal      uint64
	directoryCountTotal uint64
	symlinkCountTotal   uint64
	xattrCountTotal     uint64
	totalSize           uint64

	// counts (event-driven, no per-path memory)
	countFiles          uint64
	countFilesOk        uint64
	countFilesErrors    uint64
	countSymlinks       uint64
	countSymlinksOk     uint64
	countSymlinksErrors uint64
	countXattrs         uint64
	countXattrsOk       uint64
	countXattrsErrors   uint64

	countDirs       uint64
	countDirsOk     uint64
	countDirsErrors uint64

	// timers
	timerResourcesDone    bool
	timerResourcesBegin   time.Time
	timerResourcesElapsed time.Duration

	timerStructureDone    bool
	timerStructureBegin   time.Time
	timerStructureElapsed time.Duration

	// UI
	ressourcesProgress progress.Model
	structureProgress  progress.Model

	// ETA calculation
	lastETAAt      time.Time
	lastResDone    uint64
	lastStructDone uint64
	resRateEMA     float64 // items/sec
	structRateEMA  float64 // items/sec

	spin spinner.Model
}

func newBackupModel(ctx *appcontext.AppContext, events <-chan Event) tea.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	rpr := progress.New(progress.WithDefaultGradient())
	spr := progress.New(progress.WithDefaultGradient())

	return backupModel{
		appContext:         ctx,
		events:             events,
		startTime:          time.Now(),
		ressourcesProgress: rpr,
		structureProgress:  spr,
		spin:               sp,
	}
}

func (m backupModel) Init() tea.Cmd {
	return tea.Batch(
		m.spin.Tick,
		tick(),
		waitForDoneEvent(m.events), // exactly one waiter in flight
	)
}

func (m backupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch event := msg.(type) {
	case tickMsg:
		// throttle percent updates
		const minFrame = 100 * time.Millisecond
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)

		now := time.Now()
		if !m.timerResourcesBegin.IsZero() && !m.timerResourcesDone {
			m.timerResourcesElapsed = time.Since(m.timerResourcesBegin)
		}
		if !m.timerStructureBegin.IsZero() && !m.timerStructureDone {
			m.timerStructureElapsed = time.Since(m.timerStructureBegin)
		}

		if m.lastETAAt.IsZero() {
			m.lastETAAt = now
			m.lastResDone = m.countFilesOk + m.countFilesErrors + m.countSymlinksOk + m.countSymlinksErrors + m.countXattrsOk + m.countXattrsErrors
			m.lastStructDone = m.countDirsOk + m.countDirsErrors
			return m, tea.Batch(cmd, tick())
		}

		dt := now.Sub(m.lastETAAt).Seconds()
		if dt > 0.2 { // update rate ~5Hz max; avoids noise
			resDone := m.countFilesOk + m.countFilesErrors + m.countSymlinksOk + m.countSymlinksErrors + m.countXattrsOk + m.countXattrsErrors
			structDone := m.countDirsOk + m.countDirsErrors

			resRate := float64(resDone-m.lastResDone) / dt
			structRate := float64(structDone-m.lastStructDone) / dt

			// EMA smoothing
			const alpha = 0.2
			if resRate > 0 {
				if m.resRateEMA == 0 {
					m.resRateEMA = resRate
				} else {
					m.resRateEMA = alpha*resRate + (1-alpha)*m.resRateEMA
				}
			}
			if structRate > 0 {
				if m.structRateEMA == 0 {
					m.structRateEMA = structRate
				} else {
					m.structRateEMA = alpha*structRate + (1-alpha)*m.structRateEMA
				}
			}

			m.lastETAAt = now
			m.lastResDone = resDone
			m.lastStructDone = structDone
		}

		return m, tea.Batch(cmd, tick())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case appMsg:
		e := Event(event)

		switch e.Type {
		case "workflow.start":
			m.startTime = time.Now()
			m.phase = "preparing backup..."
			m.snapshotID = fmt.Sprintf("%x", e.Snapshot[0:4])

		case "workflow.end":
			//m.phase = "done !"

		case "directory":
			m.countDirs++

		case "file":
			m.countFiles++

		case "file.ok":
			m.countFilesOk++
			m.detail = fmt.Sprintf("%s", e.Data["path"].(string))

		case "directory.ok":
			m.countDirsOk++
			m.detail = fmt.Sprintf("%s", e.Data["path"].(string))

		case "file.error":
			m.countFilesErrors++

		case "directory.error":
			m.countDirsErrors++

		case "path.error":

		case "fs.summary":
			m.foundSummary = true
			m.fileCountTotal = e.Data["files"].(uint64)
			m.directoryCountTotal = e.Data["directories"].(uint64)
			m.symlinkCountTotal = e.Data["symlinks"].(uint64)
			m.xattrCountTotal = e.Data["xattrs"].(uint64)
			m.totalSize = e.Data["size"].(uint64)

		case "snapshot.import.start":
			m.phase = "processing resources"
			m.timerResourcesBegin = time.Now()

		case "snapshot.import.done":
			m.phase = "done importing resources"
			m.detail = ""
			m.timerResourcesDone = true

		case "snapshot.vfs.start":
			m.phase = "building virtual filesystem"
			m.timerStructureBegin = time.Now()

		case "snapshot.vfs.end":
			m.phase = "done building virtual filesystem"
			m.detail = ""
			m.timerStructureDone = true

		case "snapshot.index.start":
			m.phase = "indexing resources"

		case "snapshot.index.end":
			m.phase = "done indexing resources"
			m.detail = ""

		case "snapshot.commit.start":
			m.phase = "committing snapshot"

		case "result":
			m.phase = "published snapshot"
			m.detail = fmt.Sprintf("size=%s errors=%d duration=%s", humanize.IBytes(e.Data["size"].(uint64)), e.Data["errors"].(uint64), e.Data["duration"].(time.Duration))
		}

		// re-arm exactly one next wait
		return m, waitForDoneEvent(m.events)

	case tea.KeyMsg:
		switch event.String() {
		case "ctrl+c":
			m.forceQuit = true
			m.appContext.Cancel(fmt.Errorf("aborted by user"))
			return m, tea.Quit
		}

	case tea.QuitMsg:
		m.phase = "Aborted"
		return m, tea.Quit
	}

	return m, nil
}

func (m backupModel) View() string {
	// Don't flash UI if nothing happened
	if m.countFilesOk == 0 && m.countFilesErrors == 0 && m.countDirsOk == 0 && m.countDirsErrors == 0 && m.phase == "" {
		return ""
	}

	if m.forceQuit {
		return fmt.Sprintf("[%s] backup: aborted !\n", humanDuration(time.Since(m.startTime)))
	}

	var s strings.Builder

	detail := ""
	if m.detail != "" {
		detail = ": " + m.detail
	}

	fmt.Fprintf(&s, "[%s] %s: %s%s\n", humanDuration(time.Since(m.startTime)), m.snapshotID, m.phase, detail)

	if m.foundSummary && m.fileCountTotal+m.xattrCountTotal > 0 {
		done := m.countFilesOk + m.countFilesErrors
		total := m.fileCountTotal + m.symlinkCountTotal + m.xattrCountTotal

		// Clamp ratio to [0,1]
		ratio := float64(done) / float64(total)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}

		eta := ""
		if m.resRateEMA > 0 && done > 10 && time.Since(m.startTime) > 2*time.Second {
			remaining := float64(total - done)
			etaDur := time.Duration(remaining / m.resRateEMA * float64(time.Second))
			etaValue := fmtETA(etaDur)
			if etaValue != "" {
				eta = "ETA " + etaValue
			}
		}

		fmt.Fprintf(&s, "resources: %s %s [%d/%d] %s", m.ressourcesProgress.ViewAs(ratio), humanDuration(m.timerResourcesElapsed), done, total, eta)
		if m.countFilesErrors+m.countSymlinksErrors+m.countXattrsErrors > 0 {
			fmt.Fprintf(&s, "  %s %d", crossMark, m.countFilesErrors+m.countSymlinksErrors+m.countXattrsErrors)
		}
		fmt.Fprintf(&s, "\n")
	} else {
		fmt.Fprintf(&s, "resources: %s %s %s %d", m.spin.View(), humanDuration(m.timerResourcesElapsed), checkMark, m.countFilesOk)
		if m.countFilesErrors > 0 {
			fmt.Fprintf(&s, "  %s %d", crossMark, m.countFilesErrors)
		}
		fmt.Fprintf(&s, "\n")
	}

	if m.foundSummary && m.directoryCountTotal > 0 {
		done := m.countDirsOk + m.countDirsErrors
		total := m.directoryCountTotal

		// Clamp ratio to [0,1]
		ratio := float64(done) / float64(total)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}

		eta := ""
		if m.structRateEMA > 0 && done > 10 && time.Since(m.startTime) > 2*time.Second {
			remaining := float64(total - done)
			etaDur := time.Duration(remaining / m.structRateEMA * float64(time.Second))
			etaValue := fmtETA(etaDur)
			if etaValue != "" {
				eta = "ETA " + etaValue
			}
		}

		fmt.Fprintf(&s, "structure: %s %s [%d/%d] %s", m.structureProgress.ViewAs(ratio), humanDuration(m.timerStructureElapsed), done, total, eta)
		if m.countDirsErrors > 0 {
			fmt.Fprintf(&s, "  %s %d", crossMark, m.countDirsErrors)
		}
		fmt.Fprintf(&s, "\n")

	} else {
		fmt.Fprintf(&s, "structure: %s %s %s %d", m.spin.View(), humanDuration(m.timerStructureElapsed), checkMark, m.countDirsOk)
		if m.countDirsErrors > 0 {
			fmt.Fprintf(&s, "  %s %d", crossMark, m.countDirsErrors)
		}
		fmt.Fprintf(&s, "\n")
	}

	return s.String()
}

func fmtETA(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	// keep it short
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
