package tmux

import (
	"fmt"
	"sort"
	"strconv"
)

// equalizeGrid sizes an already-formed rectangular grid using absolute pane
// dimensions. Integer percentages leave a large remainder for grids whose
// dimensions do not divide 100 evenly (for example, six columns at 16% each).
func equalizeGrid(grid [][]string) error {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return fmt.Errorf("grid must contain at least one pane")
	}

	type positionedRow struct {
		panes []string
		top   int
	}

	cols := len(grid[0])
	rows := make([]positionedRow, len(grid))
	for row := range grid {
		if len(grid[row]) != cols {
			return fmt.Errorf("row %d has %d panes, want %d", row, len(grid[row]), cols)
		}
		panes, err := orderPanes(grid[row], "#{pane_left}")
		if err != nil {
			return fmt.Errorf("ordering row %d: %w", row, err)
		}
		top, err := paneCoordinate(panes[0], "#{pane_top}")
		if err != nil {
			return fmt.Errorf("locating row %d: %w", row, err)
		}
		rows[row] = positionedRow{panes: panes, top: top}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].top < rows[j].top
	})

	rowAnchors := make([]string, len(rows))
	for row := range rows {
		rowAnchors[row] = rows[row].panes[0]
	}

	if err := equalizePanes(rowAnchors, "#{pane_height}", "-y"); err != nil {
		return fmt.Errorf("equalizing rows: %w", err)
	}
	for row := range rows {
		if err := equalizePanes(rows[row].panes, "#{pane_width}", "-x"); err != nil {
			return fmt.Errorf("equalizing row %d: %w", row, err)
		}
	}
	return nil
}

func orderPanes(panes []string, coordinateFormat string) ([]string, error) {
	type positionedPane struct {
		id         string
		coordinate int
	}

	positioned := make([]positionedPane, len(panes))
	for i, pane := range panes {
		coordinate, err := paneCoordinate(pane, coordinateFormat)
		if err != nil {
			return nil, err
		}
		positioned[i] = positionedPane{id: pane, coordinate: coordinate}
	}
	sort.Slice(positioned, func(i, j int) bool {
		return positioned[i].coordinate < positioned[j].coordinate
	})

	ordered := make([]string, len(positioned))
	for i, pane := range positioned {
		ordered[i] = pane.id
	}
	return ordered, nil
}

func paneCoordinate(pane, coordinateFormat string) (int, error) {
	out, err := run("display-message", "-p", "-t", pane, coordinateFormat)
	if err != nil {
		return 0, err
	}
	coordinate, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("invalid pane coordinate %q for %s: %w", out, pane, err)
	}
	return coordinate, nil
}

func equalizePanes(panes []string, sizeFormat, resizeFlag string) error {
	if len(panes) < 2 {
		return nil
	}

	total := 0
	for _, pane := range panes {
		out, err := run("display-message", "-p", "-t", pane, sizeFormat)
		if err != nil {
			return err
		}
		size, err := strconv.Atoi(out)
		if err != nil {
			return fmt.Errorf("invalid pane size %q for %s: %w", out, pane, err)
		}
		total += size
	}

	base := total / len(panes)
	extra := total % len(panes)
	if base < 1 {
		return fmt.Errorf("%d panes do not fit in %d cells", len(panes), total)
	}

	// The final pane absorbs any rounding adjustment made by tmux.
	for i, pane := range panes[:len(panes)-1] {
		size := base
		if i < extra {
			size++
		}
		if _, err := run("resize-pane", "-t", pane, resizeFlag, strconv.Itoa(size)); err != nil {
			return err
		}
	}
	return nil
}
