package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"layouts/internal/config"
)

func ApplyLayout(sessionName, startDir string, layout *config.LayoutConfig) error {
	if layout == nil || len(layout.Windows) == 0 {
		return fmt.Errorf("layout has no windows")
	}
	return addWindows(sessionName, startDir, layout.Windows)
}

func CreateSessionWithLayout(name, startDir string, layout *config.LayoutConfig) error {
	if err := NewSession(name, startDir); err != nil {
		return err
	}
	if layout == nil || len(layout.Windows) == 0 {
		return nil
	}
	return applyToNewSession(name, startDir, layout.Windows)
}

func baseIndex() int {
	out, err := run("show-option", "-gv", "base-index")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func addWindows(sessionName, startDir string, windows []config.WindowConfig) error {
	for _, win := range windows {
		out, err := run("new-window", "-t", sessionName, "-n", win.Name, "-c", startDir, "-P", "-F", "#{window_index}")
		if err != nil {
			return fmt.Errorf("creating window %s: %w", win.Name, err)
		}
		winIdx, _ := strconv.Atoi(strings.TrimSpace(out))

		if err := applyPanes(sessionName, winIdx, startDir, win); err != nil {
			return err
		}
	}
	return nil
}

func applyToNewSession(sessionName, startDir string, windows []config.WindowConfig) error {
	base := baseIndex()
	var firstWinIdx int

	for i, win := range windows {
		var winIdx int
		if i == 0 {
			winIdx = base
			if _, err := run("rename-window", "-t", fmt.Sprintf("%s:%d", sessionName, winIdx), win.Name); err != nil {
				return fmt.Errorf("renaming window: %w", err)
			}
			firstWinIdx = winIdx
		} else {
			out, err := run("new-window", "-t", sessionName, "-n", win.Name, "-c", startDir, "-P", "-F", "#{window_index}")
			if err != nil {
				return fmt.Errorf("creating window %s: %w", win.Name, err)
			}
			winIdx, _ = strconv.Atoi(strings.TrimSpace(out))
		}

		if err := applyPanes(sessionName, winIdx, startDir, win); err != nil {
			return err
		}
	}

	firstWin := fmt.Sprintf("%s:%d", sessionName, firstWinIdx)
	run("select-window", "-t", firstWin)
	run("select-pane", "-t", firstWin+".0")

	return nil
}

func paneBaseIndex() int {
	out, err := run("show-option", "-gv", "pane-base-index")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func applyPanes(sessionName string, windowIdx int, startDir string, win config.WindowConfig) error {
	if len(win.Panes) <= 1 {
		if len(win.Panes) == 1 && win.Panes[0].Cmd != "" {
			sendCommand(sessionName, windowIdx, 0, win.Panes[0].Cmd)
		}
		return nil
	}

	winTarget := fmt.Sprintf("%s:%d", sessionName, windowIdx)
	paneBase := paneBaseIndex()

	if win.Rows > 1 {
		return applyGrid(sessionName, windowIdx, startDir, win, winTarget, paneBase)
	}
	return applyFlat(sessionName, windowIdx, startDir, win, winTarget, paneBase)
}

// applyGrid creates a rows × cols grid within a single window.
// Strategy: split into rows (-v), then split each row into cols (-h).
func applyGrid(sessionName string, windowIdx int, startDir string, win config.WindowConfig, winTarget string, paneBase int) error {
	rows := win.Rows
	cols := len(win.Panes) / rows

	// Step 1: create row splits. The initial pane becomes row 0.
	// Split (rows-1) times vertically to create the rows.
	// Track the pane ID of the first pane in each row.
	rowPanes := make([]string, rows)
	rowPanes[0] = fmt.Sprintf("%s:%d.%d", sessionName, windowIdx, paneBase)

	for r := 1; r < rows; r++ {
		out, err := run("split-window", "-v", "-t", rowPanes[0], "-c", startDir, "-P", "-F", "#{pane_id}")
		if err != nil {
			return fmt.Errorf("creating row %d in window %s: %w", r, win.Name, err)
		}
		rowPanes[r] = strings.TrimSpace(out)
	}

	// Resize all rows to equal height
	heightPct := 100 / rows
	for r := 0; r < rows-1; r++ {
		run("resize-pane", "-t", rowPanes[r], "-y", fmt.Sprintf("%d%%", heightPct))
	}

	// Step 2: split each row into cols horizontally.
	for r := 0; r < rows; r++ {
		for c := 1; c < cols; c++ {
			if _, err := run("split-window", "-h", "-t", rowPanes[r], "-c", startDir); err != nil {
				return fmt.Errorf("creating col %d in row %d of window %s: %w", c, r, win.Name, err)
			}
		}
		// Resize columns in this row to equal width
		// After splitting, the row's panes are at consecutive indices.
		// Re-query pane list for this row isn't needed — just resize by target.
	}

	// Step 3: resize columns evenly. After all splits, panes are indexed
	// sequentially. Row 0 has paneBase..paneBase+cols-1, row 1 has
	// paneBase+cols..paneBase+2*cols-1, etc.
	widthPct := 100 / cols
	for r := 0; r < rows; r++ {
		for c := 0; c < cols-1; c++ {
			idx := paneBase + r*cols + c
			target := fmt.Sprintf("%s:%d.%d", sessionName, windowIdx, idx)
			run("resize-pane", "-t", target, "-x", fmt.Sprintf("%d%%", widthPct))
		}
	}

	// Step 4: send commands
	for i, pane := range win.Panes {
		if pane.Cmd != "" {
			sendCommand(sessionName, windowIdx, paneBase+i, pane.Cmd)
		}
	}

	run("select-pane", "-t", fmt.Sprintf("%s:%d.%d", sessionName, windowIdx, paneBase))
	return nil
}

// applyFlat creates panes in a single direction (horizontal or vertical).
func applyFlat(sessionName string, windowIdx int, startDir string, win config.WindowConfig, winTarget string, paneBase int) error {
	sizes := computeSizes(win.Panes)

	splitFlag := "-h"
	if win.Split == "vertical" {
		splitFlag = "-v"
	}

	for i := 1; i < len(win.Panes); i++ {
		if _, err := run("split-window", splitFlag, "-t", winTarget, "-c", startDir); err != nil {
			return fmt.Errorf("splitting pane %d in window %s: %w", i, win.Name, err)
		}
		if win.Layout != "" {
			run("select-layout", "-t", winTarget, win.Layout)
		}
	}

	if win.Layout == "" {
		resizeFlag := "-x"
		if win.Split == "vertical" {
			resizeFlag = "-y"
		}
		for i := 0; i < len(sizes)-1; i++ {
			target := fmt.Sprintf("%s:%d.%d", sessionName, windowIdx, paneBase+i)
			if _, err := run("resize-pane", "-t", target, resizeFlag, fmt.Sprintf("%d%%", sizes[i])); err != nil {
				return fmt.Errorf("resizing pane %d in window %s: %w", i, win.Name, err)
			}
		}
	}

	for i, pane := range win.Panes {
		if pane.Cmd != "" {
			sendCommand(sessionName, windowIdx, paneBase+i, pane.Cmd)
		}
	}

	run("select-pane", "-t", fmt.Sprintf("%s:%d.%d", sessionName, windowIdx, paneBase))
	return nil
}

func sendCommand(sessionName string, windowIdx, paneIdx int, cmd string) {
	target := fmt.Sprintf("%s:%d.%d", sessionName, windowIdx, paneIdx)
	run("send-keys", "-t", target, "-l", cmd)
	run("send-keys", "-t", target, "Enter")
}

func computeSizes(panes []config.PaneConfig) []int {
	sizes := make([]int, len(panes))
	totalSpecified := 0
	unspecifiedCount := 0

	for i, p := range panes {
		if p.Size != "" {
			sizes[i] = parseSize(p.Size)
			totalSpecified += sizes[i]
		} else {
			unspecifiedCount++
		}
	}

	if unspecifiedCount > 0 {
		remaining := 100 - totalSpecified
		if remaining < 0 {
			remaining = 0
		}
		each := remaining / unspecifiedCount
		extra := remaining % unspecifiedCount
		for i := range sizes {
			if sizes[i] == 0 {
				sizes[i] = each
				if extra > 0 {
					sizes[i]++
					extra--
				}
			}
		}
	}

	return sizes
}

func parseSize(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	n, _ := strconv.Atoi(s)
	return n
}
