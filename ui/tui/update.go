package tui

import (
	"fmt"
	"time"

	"github.com/PlakarKorp/kloset/objects"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
)

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch event := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = event.Width
		return m, nil

	case eventsClosedMsg:
		return m, tea.Quit

	case cancelledMsg:
		m.forceQuit = true
		m.phase = "Aborted"
		return m, tea.Quit

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
			//m.lastResDone = m.countFileOk + m.countFilesErrors + m.countSymlinksOk + m.countSymlinksErrors + m.countXattrsOk + m.countXattrsErrors
			m.lastStructDone = m.countDirOk + m.countDirError
			return m, tea.Batch(cmd, tick())
		}

		dt := now.Sub(m.lastETAAt).Seconds()
		if dt > 0.2 { // update rate ~5Hz max; avoids noise
			resDone := m.countFileOk + m.countFileError + m.countSymlinkOk + m.countSymlinkError + m.countXattrOk + m.countXattrError
			structDone := m.countDirOk + m.countDirError

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
			m.snapshotID = fmt.Sprintf("%x", e.Snapshot[0:4])

		case "workflow.end":
			m.clearPathBuffer()

		case "path":
			if p, ok := e.Data["path"].(string); ok {
				m.lastPathname = p
			}
			m.countPath++

		case "path.ok":
			if p, ok := e.Data["path"].(string); ok {
				m.lastPathname = p
				m.pushPathLine(fmt.Sprintf("%s %s", checkMark, p))
			}
			m.countPathOk++

		case "path.error":
			if p, ok := e.Data["path"].(string); ok {
				m.lastPathname = p
				m.pushPathLine(fmt.Sprintf("%s %s", crossMark, p))
				m.pushErrorLine(fmt.Sprintf("%s %s", crossMark, p))
			}
			m.countPathError++

		case "directory":
			m.countDir++

		case "directory.ok":
			m.countDirOk++

		case "directory.error":
			m.countDirError++

		case "file":
			m.countFile++

		case "file.ok":
			m.countFileOk++
			fileinfo := e.Data["fileinfo"].(objects.FileInfo)
			m.countFileSize += fileinfo.Size()

		case "file.error":
			m.countFileError++

		case "xattr":
			m.countXattr++

		case "xattr.ok":
			m.countXattrOk++
			size := e.Data["size"].(int64)
			m.countXattrSize += size

		case "xattr.error":
			m.countXattrError++

		case "symlink":
			m.countSymlink++

		case "symlink.ok":
			m.countSymlinkOk++

		case "symlink.error":
			m.countSymlinkError++

		case "fs.summary":
			m.foundSummary = true
			m.fileCountTotal = e.Data["files"].(uint64)
			m.directoryCountTotal = e.Data["directories"].(uint64)
			m.symlinkCountTotal = e.Data["symlinks"].(uint64)
			m.xattrCountTotal = e.Data["xattrs"].(uint64)
			m.totalSize = e.Data["size"].(uint64)
			m.summaryPathTotal = m.fileCountTotal + m.directoryCountTotal + m.symlinkCountTotal + m.xattrCountTotal

		case "snapshot.import.start":
			m.phase = "processing"
			m.timerResourcesBegin = time.Now()

		case "snapshot.import.done":
			m.detail = ""
			m.timerResourcesDone = true
			m.lastPathname = "DONE !"

		case "snapshot.vfs.start":
			m.phase = "building VFS"
			m.timerStructureBegin = time.Now()

		case "snapshot.vfs.end":
			m.phase = ""
			m.detail = ""
			m.timerStructureDone = true

		case "snapshot.index.start":
			m.phase = "indexing"

		case "snapshot.index.end":
			m.phase = ""
			m.detail = ""

		case "snapshot.commit.start":
			m.phase = "committing"

		case "result":
			m.clearPathBuffer()
			m.phase = "DONE !"
			m.detail = fmt.Sprintf("size=%s errors=%d duration=%s", humanize.IBytes(e.Data["size"].(uint64)), e.Data["errors"].(uint64), e.Data["duration"].(time.Duration))
		}

		// re-arm exactly one next wait
		m.dirty = true
		return m, waitForDoneEvent(m.events)

	case tea.KeyMsg:
		switch event.String() {
		case "ctrl+c":
			m.forceQuit = true
			return m, tea.Interrupt
		}

	case tea.QuitMsg:
		m.phase = "Aborted"
		return m, tea.Quit
	}

	return m, nil
}

func (m *appModel) pushErrorLine(line string) {
	if m.lastNErrors <= 0 {
		return
	}
	if len(m.lastErrors) < m.lastNErrors {
		m.lastErrors = append(m.lastErrors, line)
		return
	}
	// drop oldest, keep last N
	copy(m.lastErrors, m.lastErrors[1:])
	m.lastErrors[len(m.lastErrors)-1] = line
}

func (m *appModel) clearErrorBuffer() {
	m.lastNErrors = 0
	m.lastErrors = m.lastErrors[:0]
}

func (m *appModel) pushPathLine(line string) {
	if m.lastNPaths <= 0 {
		return
	}
	if len(m.lastPaths) < m.lastNPaths {
		m.lastPaths = append(m.lastPaths, line)
		return
	}
	// drop oldest, keep last N
	copy(m.lastPaths, m.lastPaths[1:])
	m.lastPaths[len(m.lastPaths)-1] = line
}

func (m *appModel) clearPathBuffer() {
	m.lastNPaths = 0
	m.lastPaths = m.lastPaths[:0]
}
