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
		m.height = event.Height
		return m, nil

	case eventsClosedMsg:
		return m, tea.Quit

	case cancelledMsg:
		m.forceQuit = true
		m.phase = "Aborted"
		return m, tea.Quit

	case tickMsg:
		// keep spinner + ETA updates responsive
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
			m.lastStructDone = m.countDirOk + m.countDirError
		} else {
			dt := now.Sub(m.lastETAAt).Seconds()
			if dt > 0.2 { // ~5Hz max
				resDone := m.countFileOk + m.countFileError + m.countSymlinkOk + m.countSymlinkError + m.countXattrOk + m.countXattrError
				structDone := m.countDirOk + m.countDirError

				resRate := float64(resDone-m.lastResDone) / dt
				structRate := float64(structDone-m.lastStructDone) / dt

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
		}

		// IMPORTANT: do NOT re-arm waitForBatch here; keep exactly one waiter in flight
		// (armed after processing each appBatchMsg)
		return m, tea.Batch(cmd, tick())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)

		// same: don't re-arm waitForBatch here
		return m, cmd

	case appBatchMsg:
		// process a whole batch of events in one Bubbletea update
		for _, e := range event.events {
			switch e.Type {
			case "workflow.start":
				m.startTime = time.Now()
				m.snapshotID = fmt.Sprintf("%x", e.Snapshot[0:4])

			case "workflow.end":

			case "path":
				if p, ok := e.Data["path"].(string); ok {
					m.lastItem = p
				}
				m.countPath++

			case "path.ok":
				m.countPathOk++

			case "path.error":
				if p, ok := e.Data["path"].(string); ok {
					m.errors = append(m.errors, fmt.Sprintf("%s %s: %s", crossMark, p, e.Data["error"]))
				}
				m.countPathError++

			case "path.cached":
				m.countPathCached++

			case "directory":
				m.countDir++

			case "directory.ok":
				m.countDirOk++

			case "directory.error":
				m.countDirError++

			case "directory.cached":
				m.countDirCached++

			case "file":
				m.countFile++

			case "file.ok":
				m.countFileOk++
				fileinfo := e.Data["fileinfo"].(objects.FileInfo)
				m.countFileSize += fileinfo.Size()

			case "file.error":
				m.countFileError++

			case "file.cached":
				m.countFileCached++
				fileinfo := e.Data["fileinfo"].(objects.FileInfo)
				m.countCachedSize += uint64(fileinfo.Size())

			case "xattr":
				m.countXattr++

			case "xattr.ok":
				m.countXattrOk++
				size := e.Data["size"].(int64)
				m.countXattrSize += size

			case "xattr.error":
				m.countXattrError++

			case "xattr.cached":
				m.countXattrCached++
				size := e.Data["size"].(int64)
				m.countCachedSize += uint64(size)

			case "symlink":
				m.countSymlink++

			case "symlink.ok":
				m.countSymlinkOk++

			case "symlink.error":
				m.countSymlinkError++

			case "symlink.cached":
				m.countSymlinkCached++

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

			case "snapshot.vfs.start":
				m.lastItem = ""
				m.phase = "building VFS"
				m.timerStructureBegin = time.Now()

			case "snapshot.vfs.end":
				m.phase = ""
				m.detail = ""
				m.lastItem = ""
				m.timerStructureDone = true

			case "snapshot.index.start":
				m.lastItem = ""
				m.phase = "indexing"

			case "snapshot.index.end":
				m.lastItem = ""
				m.phase = ""
				m.detail = ""

			case "snapshot.commit.start":
				m.lastItem = ""
				m.phase = "committing"

			case "result":
				m.lastItem = ""
				m.phase = "completed"
				m.detail = fmt.Sprintf(
					"size=%s errors=%d duration=%s",
					humanize.IBytes(e.Data["size"].(uint64)),
					e.Data["errors"].(uint64),
					e.Data["duration"].(time.Duration),
				)
			}
		}

		m.dirty = true

		// re-arm exactly one next waiter (batch)
		const batchMax = 512
		return m, waitForBatch(m.events, batchMax)

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
