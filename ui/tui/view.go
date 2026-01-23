package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/muesli/termenv"
)

var (
	checkMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).SetString("✓")
	crossMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).SetString("✘")
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))             // green
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))             // gray (optional)
	reuseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true) // bright green
	newStyle   = okStyle
)

func ok(n uint64) string  { return okStyle.Render(fmt.Sprintf("%d", n)) }
func err(n uint64) string { return errStyle.Render(fmt.Sprintf("%d", n)) }
func tot(n uint64) string { return dimStyle.Render(fmt.Sprintf("%d", n)) } // or plain fmt.Sprintf

func okSize(n uint64) string  { return okStyle.Render(fmt.Sprintf("%s", humanize.IBytes(n))) }
func errSize(n uint64) string { return errStyle.Render(fmt.Sprintf("%s", humanize.IBytes(n))) }
func totSize(n uint64) string { return dimStyle.Render(fmt.Sprintf("%s", humanize.IBytes(n))) } // or plain fmt.Sprintf

func fmtCount(ok, err, total uint64) string {
	okS := okStyle.Render(fmt.Sprintf("%d", ok))
	totalS := dimStyle.Render(fmt.Sprintf("%d", total))

	if err == 0 {
		// ok/total
		return fmt.Sprintf("%s/%s", okS, totalS)
	}

	errS := errStyle.Render(fmt.Sprintf("%d", err))
	// ok/error/total
	return fmt.Sprintf("%s/%s/%s", okS, errS, totalS)
}

func humanDuration(d time.Duration) string {
	sec := int(d.Round(time.Second).Seconds())
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func progressBar() progress.Model {
	//	return progress.New(progress.WithDefaultGradient())

	p := progress.New(
		progress.WithColorProfile(termenv.Ascii),
	)
	// Make it ASCII-ish
	p.Full = '*' // #
	p.Empty = ' '
	//	p.Left = '['
	//	p.Right = ']'
	//	p.Head = '>' // moving head, like many CLIs
	p.ShowPercentage = true
	//p.Width = 100

	return p
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

func formatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	return humanize.IBytes(uint64(b))
}

func formatThroughput(bps float64) string {
	if bps <= 0 || math.IsNaN(bps) || math.IsInf(bps, 0) {
		return "0 B/s"
	}
	return fmt.Sprintf("%s/s", humanize.IBytes(uint64(bps)))
}

func fmtCountCached(ok, errc, total, cached uint64) string {
	base := fmtCount(ok, errc, total) // your existing "ok/total" (or however it prints)
	if cached == 0 {
		return base
	}
	return fmt.Sprintf("%s (%s cached)", base, tot(cached))
}

func pct(num, den uint64) string {
	if den == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", (float64(num)*100)/float64(den))
}

func fmtNewReuse(okCount, cached, errc, total uint64) string {
	okS := okStyle.Render(fmt.Sprintf("%d", okCount))
	totalS := dimStyle.Render(fmt.Sprintf("%d", total))

	var base string
	if errc == 0 {
		base = fmt.Sprintf("%s/%s", okS, totalS)
	} else {
		errS := errStyle.Render(fmt.Sprintf("%d", errc))
		base = fmt.Sprintf("%s/%s/%s", okS, errS, totalS)
	}

	if cached == 0 {
		return base
	}

	// cache counter displayed AFTER the total
	//cachedS := reuseStyle.Render(fmt.Sprintf("(%d reused)", cached))
	//return fmt.Sprintf("%s %s", base, cachedS)
	return fmt.Sprintf("%s", base)
}

func (m appModel) View() string {
	if !m.dirty {
		return ""
	}
	m.dirty = false

	if m.forceQuit {
		return fmt.Sprintf("[%s] %s: aborted !\n", humanDuration(time.Since(m.startTime)), m.appName)
	}

	m.barPrefix = m.phase

	var s strings.Builder

	done := m.countPathOk + m.countPathError

	// helpers
	writeLastPaths := func() {
		if m.lastNPaths <= 0 {
			return
		}

		fmt.Fprintf(&s, "\n")
		for i := 0; i < m.lastNPaths; i++ {
			if i < len(m.lastPaths) {
				fmt.Fprintf(&s, "%s\n", m.lastPaths[i])
			} else {
				fmt.Fprintf(&s, "\n")
			}
		}
	}

	writeProcessedSummary := func() {
		nodesTotal := m.countDir
		leavesTotal := m.countFile
		symlinksTotal := m.countSymlink
		xattrsTotal := m.countXattr
		sizeTotal := uint64(m.countFileSize)
		if m.foundSummary && m.summaryPathTotal > 0 {
			leavesTotal = m.fileCountTotal
			nodesTotal = m.directoryCountTotal
			symlinksTotal = m.symlinkCountTotal
			xattrsTotal = m.xattrCountTotal
			sizeTotal = m.totalSize
		}

		indent := strings.Repeat(" ", len(humanDuration(time.Since(m.startTime))))
		fmt.Fprintf(&s, "%s   %s:", indent, m.appName)

		fmt.Fprintf(&s, " nodes=%s", fmtNewReuse(m.countDirOk, m.countDirCached, m.countDirError, nodesTotal))
		fmt.Fprintf(&s, ", leaves=%s", fmtNewReuse(m.countFileOk, m.countFileCached, m.countFileError, leavesTotal))

		if symlinksTotal != 0 {
			fmt.Fprintf(&s, ", links=%s",
				fmtNewReuse(m.countSymlinkOk, m.countSymlinkCached, m.countSymlinkError, symlinksTotal),
			)
		}
		if xattrsTotal != 0 {
			fmt.Fprintf(&s, ", attrs=%s",
				fmtNewReuse(m.countXattrOk, m.countXattrCached, m.countXattrError, xattrsTotal),
			)
		}
		fmt.Fprintf(&s, ", size=%s/%s", okSize(uint64(m.countFileSize)), totSize(sizeTotal))

		if m.countPathError != 0 {
			// If you want total paths from summary when available, swap denominator accordingly
			pathsTotal := m.countPath
			if m.foundSummary && m.summaryPathTotal > 0 {
				pathsTotal = m.summaryPathTotal
			}
			fmt.Fprintf(&s, ", errors=%s/%s", err(m.countPathError), tot(pathsTotal))
		}

		fmt.Fprintf(&s, "\n")
	}

	writeStoreSummary := func() {
		if m.repo == nil {
			return
		}
		ioStats := m.repo.IOStats()

		indent := strings.Repeat(" ", len(humanDuration(time.Since(m.startTime))))
		r := ioStats.Read.Stats()
		w := ioStats.Write.Stats()
		fmt.Fprintf(&s,
			"%s   kloset: read=%s, write=%s\n",
			indent,
			formatBytes(r.TotalBytes),
			formatBytes(w.TotalBytes),
		)
	}

	m.barPrefix = fmt.Sprintf("[%s] %s %s", humanDuration(time.Since(m.startTime)), m.snapshotID, m.phase)

	if m.foundSummary && m.summaryPathTotal > 0 {
		total := m.summaryPathTotal

		// Clamp ratio to [0,1]
		ratio := float64(done) / float64(total)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}

		// ---- ETA (fixed slot so layout doesn't jump) ----
		etaText := ""
		if m.resRateEMA > 0 && done > 10 && time.Since(m.startTime) > 2*time.Second && total >= done {
			remaining := float64(total - done)
			etaDur := time.Duration(remaining / m.resRateEMA * float64(time.Second))
			if v := fmtETA(etaDur); v != "" {
				etaText = "ETA " + v
			}
		}
		const etaSlotWidth = 12
		etaField := fmt.Sprintf("%-*s", etaSlotWidth, etaText)

		// ---- elapsed (fixed slot too) ----
		const elapsedSlotWidth = 8
		elapsedField := fmt.Sprintf("%-*s", elapsedSlotWidth, humanDuration(m.timerResourcesElapsed))

		// ---- prefix in front of the bar (fixed slot to avoid jumping) ----
		const prefixSlotWidth = 14 // tune for your labels
		prefix := fmt.Sprintf("%-*s", prefixSlotWidth, m.barPrefix)

		// ---- tail (elapsed + bytes + ETA + errors) ----
		tail := fmt.Sprintf("%s  %s  %s",
			elapsedField,
			humanize.IBytes(uint64(m.countFileSize)),
			etaField,
		)

		// ---- bar: full-width (fills remaining space) ----
		if m.width > 0 {
			const gap = 1

			prefixW := lipgloss.Width(prefix)
			tailW := lipgloss.Width(tail)

			barW := m.width - prefixW - gap - tailW - gap
			if barW < 10 {
				barW = 10
			}

			p := m.ressourcesProgress
			p.Width = barW
			bar := p.ViewAs(ratio)

			fmt.Fprintf(&s, "%s%s%s%s%s\n",
				prefix,
				strings.Repeat(" ", gap),
				bar,
				strings.Repeat(" ", gap),
				tail,
			)
		} else {
			bar := m.ressourcesProgress.ViewAs(ratio)
			fmt.Fprintf(&s, "%s %s %s\n", prefix, bar, tail)
		}

		writeProcessedSummary()
		writeStoreSummary()

		writeLastPaths()
		return s.String()
	}

	fmt.Fprintf(&s, "[%s] %s %s", humanDuration(m.timerResourcesElapsed), m.snapshotID, m.phase)
	fmt.Fprintf(&s, "\n")

	writeProcessedSummary()
	writeStoreSummary()
	writeLastPaths()
	return s.String()
}
