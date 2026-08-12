package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"layouts/internal/config"
)

func TestArrangeGridFromSinglePaneCreatesEvenSixByTwoGrid(t *testing.T) {
	if os.Getenv("LAYOUTS_GRID_HELPER") == "1" {
		runArrangeGridHelper()
		return
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	socket := fmt.Sprintf("layouts-grid-test-%d", os.Getpid())
	session := "grid"
	donePath := filepath.Join(t.TempDir(), "done")

	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
	})

	helper := fmt.Sprintf(
		"LAYOUTS_GRID_HELPER=1 LAYOUTS_GRID_DONE=%s %s -test.run=TestArrangeGridFromSinglePaneCreatesEvenSixByTwoGrid",
		shellQuote(donePath),
		shellQuote(os.Args[0]),
	)
	if out, err := exec.Command("tmux", "-f", "/dev/null", "-L", socket, "new-session", "-d", "-x", "510", "-y", "72", "-s", session, helper).CombinedOutput(); err != nil {
		t.Fatalf("starting tmux helper: %s: %v", strings.TrimSpace(string(out)), err)
	}

	result := waitForFile(t, donePath)
	if result != "ok" {
		t.Fatalf("ArrangeGrid failed: %s", result)
	}

	winOut, err := exec.Command("tmux", "-L", socket, "list-windows", "-t", session, "-F", "#{window_index}").Output()
	if err != nil {
		t.Fatalf("listing windows: %v", err)
	}
	win := strings.TrimSpace(strings.Split(strings.TrimSpace(string(winOut)), "\n")[0])

	out, err := exec.Command("tmux", "-L", socket, "list-panes", "-t", session+":"+win, "-F", "#{pane_left},#{pane_top},#{pane_width},#{pane_height}").Output()
	if err != nil {
		t.Fatalf("listing panes: %v", err)
	}
	panes := parsePaneGeometries(t, string(out))
	assertEvenGrid(t, panes, 6, 2)
}

func TestApplyLayoutCreatesEvenSixByTwoGrid(t *testing.T) {
	if os.Getenv("LAYOUTS_APPLY_GRID_HELPER") == "1" {
		runApplyGridHelper()
		return
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	socket := fmt.Sprintf("layouts-apply-grid-test-%d", os.Getpid())
	session := "apply-grid"
	donePath := filepath.Join(t.TempDir(), "done")

	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
	})

	helper := fmt.Sprintf(
		"LAYOUTS_APPLY_GRID_HELPER=1 LAYOUTS_GRID_DONE=%s %s -test.run=TestApplyLayoutCreatesEvenSixByTwoGrid",
		shellQuote(donePath),
		shellQuote(os.Args[0]),
	)
	if out, err := exec.Command("tmux", "-f", "/dev/null", "-L", socket, "new-session", "-d", "-x", "510", "-y", "72", "-s", session, helper).CombinedOutput(); err != nil {
		t.Fatalf("starting tmux helper: %s: %v", strings.TrimSpace(string(out)), err)
	}

	result := waitForFile(t, donePath)
	if result != "ok" {
		t.Fatalf("ApplyLayout failed: %s", result)
	}

	out, err := exec.Command("tmux", "-L", socket, "list-panes", "-t", session+":grid", "-F", "#{pane_left},#{pane_top},#{pane_width},#{pane_height}").Output()
	if err != nil {
		t.Fatalf("listing panes: %v", err)
	}
	panes := parsePaneGeometries(t, string(out))
	assertEvenGrid(t, panes, 6, 2)
}

func assertEvenGrid(t *testing.T, panes []paneGeometry, cols, rowCount int) {
	t.Helper()

	if len(panes) != cols*rowCount {
		t.Fatalf("pane count = %d, want %d; panes: %+v", len(panes), cols*rowCount, panes)
	}

	rows := panesByTop(panes)
	if len(rows) != rowCount {
		t.Fatalf("row count = %d, want %d; panes: %+v", len(rows), rowCount, panes)
	}

	tops := make([]int, 0, len(rows))
	minHeight, maxHeight := panes[0].Height, panes[0].Height
	for top, row := range rows {
		tops = append(tops, top)
		if len(row) != cols {
			t.Fatalf("row at y=%d has %d panes, want %d; row: %+v", top, len(row), cols, row)
		}
		sort.Slice(row, func(i, j int) bool { return row[i].Left < row[j].Left })
		minWidth, maxWidth := row[0].Width, row[0].Width
		for _, pane := range row {
			if pane.Width < minWidth {
				minWidth = pane.Width
			}
			if pane.Width > maxWidth {
				maxWidth = pane.Width
			}
			if pane.Height < minHeight {
				minHeight = pane.Height
			}
			if pane.Height > maxHeight {
				maxHeight = pane.Height
			}
		}
		if maxWidth-minWidth > 1 {
			t.Fatalf("row at y=%d is not evenly sized; widths range from %d to %d: %+v", top, minWidth, maxWidth, row)
		}
	}
	if maxHeight-minHeight > 1 {
		t.Fatalf("rows are not evenly sized; heights range from %d to %d: %+v", minHeight, maxHeight, panes)
	}

	sort.Ints(tops)
	firstRow := rows[tops[0]]
	sort.Slice(firstRow, func(i, j int) bool { return firstRow[i].Left < firstRow[j].Left })
	for _, top := range tops[1:] {
		row := rows[top]
		sort.Slice(row, func(i, j int) bool { return row[i].Left < row[j].Left })
		for col := range row {
			if row[col].Left != firstRow[col].Left || row[col].Width != firstRow[col].Width {
				t.Fatalf("column %d is not aligned across rows: first=%+v y=%d=%+v", col, firstRow[col], top, row[col])
			}
		}
	}
}

func runArrangeGridHelper() {
	donePath := os.Getenv("LAYOUTS_GRID_DONE")
	before, created, err := ArrangeGrid(6, 2)
	result := "ok"
	if err != nil {
		result = err.Error()
	} else if before != 1 || created != 11 {
		result = fmt.Sprintf("counts = before %d, created %d; want before 1, created 11", before, created)
	}
	_ = os.WriteFile(donePath, []byte(result), 0644)
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func runApplyGridHelper() {
	donePath := os.Getenv("LAYOUTS_GRID_DONE")
	panes := make([]config.PaneConfig, 12)
	layout := &config.LayoutConfig{
		Windows: []config.WindowConfig{{
			Name:  "grid",
			Rows:  2,
			Panes: panes,
		}},
	}
	err := ApplyLayout("apply-grid", ".", layout)
	result := "ok"
	if err != nil {
		result = err.Error()
	}
	_ = os.WriteFile(donePath, []byte(result), 0644)
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func waitForFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
	return ""
}

type paneGeometry struct {
	Left   int
	Top    int
	Width  int
	Height int
}

func parsePaneGeometries(t *testing.T, out string) []paneGeometry {
	t.Helper()
	var panes []paneGeometry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) != 4 {
			t.Fatalf("invalid pane geometry %q", line)
		}
		values := make([]int, 4)
		for i, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil {
				t.Fatalf("invalid pane geometry value %q: %v", part, err)
			}
			values[i] = n
		}
		panes = append(panes, paneGeometry{
			Left:   values[0],
			Top:    values[1],
			Width:  values[2],
			Height: values[3],
		})
	}
	return panes
}

func panesByTop(panes []paneGeometry) map[int][]paneGeometry {
	rows := make(map[int][]paneGeometry)
	for _, pane := range panes {
		rows[pane.Top] = append(rows[pane.Top], pane)
	}
	return rows
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
