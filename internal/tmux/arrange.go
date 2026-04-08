package tmux

import (
	"fmt"
	"strings"
)

// ArrangeGrid rearranges the panes in the current tmux window into a
// cols × rows grid. Existing pane content is preserved. If the window has
// fewer than cols*rows panes, empty panes are created to fill the grid.
// If it has more, ArrangeGrid returns an error rather than silently
// killing panes.
func ArrangeGrid(cols, rows int) (int, int, error) {
	if cols < 1 || rows < 1 {
		return 0, 0, fmt.Errorf("cols and rows must be >= 1")
	}
	target := cols * rows

	out, err := run("list-panes", "-F", "#{pane_id}")
	if err != nil {
		return 0, 0, fmt.Errorf("listing panes: %w", err)
	}
	panes := splitLines(out)
	n := len(panes)

	if n > target {
		return n, 0, fmt.Errorf("window has %d panes but %dx%d grid holds only %d — close panes first", n, cols, rows, target)
	}

	created := 0
	for len(panes) < target {
		out, err := run("split-window", "-d", "-h", "-t", panes[0], "-P", "-F", "#{pane_id}")
		if err != nil {
			return n, created, fmt.Errorf("creating empty pane: %w", err)
		}
		panes = append(panes, strings.TrimSpace(out))
		created++
	}

	anchor := panes[0]
	others := panes[1:]

	for _, pid := range others {
		if _, err := run("break-pane", "-d", "-s", pid); err != nil {
			return n, created, fmt.Errorf("detaching pane %s: %w", pid, err)
		}
	}

	// grid[r] holds pane IDs in row r from left to right.
	grid := make([][]string, rows)
	grid[0] = []string{anchor}

	used := 0
	for r := 1; r < rows; r++ {
		src := others[used]
		below := grid[r-1][0]
		if _, err := run("join-pane", "-v", "-s", src, "-t", below); err != nil {
			return n, created, fmt.Errorf("joining row %d: %w", r, err)
		}
		grid[r] = []string{src}
		used++
	}

	for r := 0; r < rows; r++ {
		for c := 1; c < cols; c++ {
			src := others[used]
			left := grid[r][c-1]
			if _, err := run("join-pane", "-h", "-s", src, "-t", left); err != nil {
				return n, created, fmt.Errorf("joining row %d col %d: %w", r, c, err)
			}
			grid[r] = append(grid[r], src)
			used++
		}
	}

	if rows > 1 {
		rowPct := 100 / rows
		for r := 0; r < rows-1; r++ {
			run("resize-pane", "-t", grid[r][0], "-y", fmt.Sprintf("%d%%", rowPct))
		}
	}
	if cols > 1 {
		colPct := 100 / cols
		for r := 0; r < rows; r++ {
			for c := 0; c < cols-1; c++ {
				run("resize-pane", "-t", grid[r][c], "-x", fmt.Sprintf("%d%%", colPct))
			}
		}
	}

	run("select-pane", "-t", anchor)
	return n, created, nil
}

func splitLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
