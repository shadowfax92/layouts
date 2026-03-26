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

	sizes := computeSizes(win.Panes)

	splitFlag := "-h"
	if win.Split == "vertical" {
		splitFlag = "-v"
	}

	// Create all panes via simple splits (no size — just get them to exist)
	winTarget := fmt.Sprintf("%s:%d", sessionName, windowIdx)
	for i := 1; i < len(win.Panes); i++ {
		if _, err := run("split-window", splitFlag, "-t", winTarget, "-c", startDir); err != nil {
			return fmt.Errorf("splitting pane %d in window %s: %w", i, win.Name, err)
		}
	}

	paneBase := paneBaseIndex()

	if win.Layout != "" {
		// Apply a tmux layout algorithm (e.g. "tiled" for grids) instead of manual resize
		if _, err := run("select-layout", "-t", winTarget, win.Layout); err != nil {
			return fmt.Errorf("applying layout %s to window %s: %w", win.Layout, win.Name, err)
		}
	} else {
		// Resize each pane to its exact target percentage (skip last — it takes the remainder)
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

	// Send commands
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
