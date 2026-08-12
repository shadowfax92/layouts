package herdr

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type runner func(args ...string) ([]byte, error)

type client struct {
	run runner
}

type rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type layoutPane struct {
	PaneID string `json:"pane_id"`
	Rect   rect   `json:"rect"`
}

type layoutSnapshot struct {
	WorkspaceID string       `json:"workspace_id"`
	TabID       string       `json:"tab_id"`
	Zoomed      bool         `json:"zoomed"`
	FocusedPane string       `json:"focused_pane_id"`
	Panes       []layoutPane `json:"panes"`
}

type paneInfo struct {
	PaneID        string `json:"pane_id"`
	CWD           string `json:"cwd"`
	ForegroundCWD string `json:"foreground_cwd"`
}

type tabInfo struct {
	TabID   string `json:"tab_id"`
	Focused bool   `json:"focused"`
}

func IsInsideHerdr() bool {
	return os.Getenv("HERDR_ENV") == "1" && os.Getenv("HERDR_PANE_ID") != ""
}

func ArrangeGrid(cols, rows int) (int, int, error) {
	return newClient().arrangeGrid(cols, rows)
}

func newClient() *client {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	return &client{run: commandRunner(bin)}
}

func commandRunner(bin string) runner {
	return func(args ...string) ([]byte, error) {
		output, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = err.Error()
			}
			return nil, fmt.Errorf("herdr %s: %s", strings.Join(args, " "), message)
		}
		return output, nil
	}
}

func (c *client) arrangeGrid(cols, rows int) (int, int, error) {
	if cols < 1 || rows < 1 || cols > math.MaxInt/rows {
		return 0, 0, fmt.Errorf("cols and rows must be >= 1")
	}
	target := cols * rows

	layout, err := c.currentLayout()
	if err != nil {
		return 0, 0, err
	}
	panes := orderedPanes(layout.Panes)
	before := len(panes)
	if before == 0 {
		return 0, 0, fmt.Errorf("current Herdr tab has no panes")
	}
	if before > target {
		return before, 0, fmt.Errorf("tab has %d panes but %dx%d grid holds only %d — close panes first", before, cols, rows, target)
	}
	if before == 1 && target == 1 {
		return before, 0, nil
	}
	if !containsPane(panes, layout.FocusedPane) {
		return before, 0, fmt.Errorf("focused pane %s is absent from the current Herdr layout", layout.FocusedPane)
	}

	focusedPane, err := c.pane(layout.FocusedPane)
	if err != nil {
		return before, 0, fmt.Errorf("reading focused Herdr pane: %w", err)
	}
	cwd := focusedPane.ForegroundCWD
	if cwd == "" {
		cwd = focusedPane.CWD
	}
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return before, 0, fmt.Errorf("reading current directory: %w", err)
		}
	}

	sourceTab, err := c.tab(layout.TabID)
	if err != nil {
		return before, 0, fmt.Errorf("reading current Herdr tab: %w", err)
	}

	holdingTab := ""
	holdingRoot := ""
	if before > 1 {
		holdingTab, holdingRoot, err = c.createTab(layout.WorkspaceID, cwd, "layouts-grid-holding")
		if err != nil {
			return before, 0, fmt.Errorf("creating temporary Herdr tab: %w", err)
		}
	}

	if layout.Zoomed {
		if err := c.zoomOff(layout.FocusedPane); err != nil {
			failure := fmt.Errorf("unzooming current Herdr tab: %w", err)
			if holdingTab != "" {
				failure = c.cleanupTarget(holdingTab, failure)
			}
			return before, 0, failure
		}
	}

	for i, pane := range panes[1:] {
		if _, err := c.movePane(pane.PaneID, holdingTab, holdingRoot, false); err != nil {
			return before, 0, fmt.Errorf("staging Herdr pane %s: %w; failed after %d confirmed moves, all live panes remain running, and temporary tab %s was left in place", pane.PaneID, err, i, holdingTab)
		}
	}

	placeholders, err := c.buildGrid(panes[0].PaneID, cols, rows, cwd)
	if err != nil {
		failure := fmt.Errorf("building Herdr grid: %w", err)
		if holdingTab != "" {
			failure = fmt.Errorf("%w; staged panes remain running in temporary tab %s", failure, holdingTab)
		}
		return before, 0, failure
	}

	for i, pane := range panes[1:] {
		focus := sourceTab.Focused && pane.PaneID == layout.FocusedPane
		if _, err := c.movePane(pane.PaneID, layout.TabID, placeholders[i+1], focus); err != nil {
			return before, 0, fmt.Errorf("restoring Herdr pane %s into grid: %w; failed after %d confirmed restores, all live panes remain running, and temporary tab %s was left in place", pane.PaneID, err, i, holdingTab)
		}
		if err := c.closePane(placeholders[i+1]); err != nil {
			return before, 0, fmt.Errorf("removing Herdr grid placeholder after restoring %d panes: %w; remaining panes are safe in temporary tab %s", i+1, err, holdingTab)
		}
	}

	if holdingTab != "" {
		if err := c.closeTab(holdingTab); err != nil {
			return before, target - before, fmt.Errorf("closing temporary Herdr tab after arranging grid: %w", err)
		}
	}
	if sourceTab.Focused {
		if err := c.focusTab(layout.TabID); err != nil {
			return before, target - before, fmt.Errorf("focusing arranged Herdr tab: %w", err)
		}
	}
	return before, target - before, nil
}

func (c *client) buildGrid(rootPane string, cols, rows int, cwd string) ([]string, error) {
	rowPanes, err := c.splitEven(rootPane, rows, "down", cwd)
	if err != nil {
		return nil, err
	}
	grid := make([]string, 0, cols*rows)
	for _, rowPane := range rowPanes {
		panes, err := c.splitEven(rowPane, cols, "right", cwd)
		if err != nil {
			return nil, err
		}
		grid = append(grid, panes...)
	}
	return grid, nil
}

func (c *client) splitEven(paneID string, count int, direction, cwd string) ([]string, error) {
	if count == 1 {
		return []string{paneID}, nil
	}
	leftCount := (count + 1) / 2
	rightCount := count - leftCount
	ratio := float64(leftCount) / float64(count)
	rightPane, err := c.splitPane(paneID, direction, ratio, cwd)
	if err != nil {
		return nil, err
	}
	left, err := c.splitEven(paneID, leftCount, direction, cwd)
	if err != nil {
		return nil, err
	}
	right, err := c.splitEven(rightPane, rightCount, direction, cwd)
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func (c *client) currentLayout() (layoutSnapshot, error) {
	output, err := c.run("pane", "layout", "--current")
	if err != nil {
		return layoutSnapshot{}, err
	}
	var response struct {
		Result struct {
			Layout layoutSnapshot `json:"layout"`
		} `json:"result"`
	}
	if err := decode(output, &response); err != nil {
		return layoutSnapshot{}, fmt.Errorf("parsing `herdr pane layout`: %w", err)
	}
	return response.Result.Layout, nil
}

func (c *client) pane(paneID string) (paneInfo, error) {
	output, err := c.run("pane", "get", paneID)
	if err != nil {
		return paneInfo{}, err
	}
	var response struct {
		Result struct {
			Pane paneInfo `json:"pane"`
		} `json:"result"`
	}
	if err := decode(output, &response); err != nil {
		return paneInfo{}, fmt.Errorf("parsing `herdr pane get`: %w", err)
	}
	return response.Result.Pane, nil
}

func (c *client) tab(tabID string) (tabInfo, error) {
	output, err := c.run("tab", "get", tabID)
	if err != nil {
		return tabInfo{}, err
	}
	var response struct {
		Result struct {
			Tab tabInfo `json:"tab"`
		} `json:"result"`
	}
	if err := decode(output, &response); err != nil {
		return tabInfo{}, fmt.Errorf("parsing `herdr tab get`: %w", err)
	}
	return response.Result.Tab, nil
}

func (c *client) createTab(workspaceID, cwd, label string) (string, string, error) {
	args := []string{"tab", "create", "--workspace", workspaceID, "--cwd", cwd}
	if label != "" {
		args = append(args, "--label", label)
	}
	args = append(args, "--no-focus")
	output, err := c.run(args...)
	if err != nil {
		return "", "", err
	}
	var response struct {
		Result struct {
			RootPane paneInfo `json:"root_pane"`
			Tab      tabInfo  `json:"tab"`
		} `json:"result"`
	}
	if err := decode(output, &response); err != nil {
		return "", "", fmt.Errorf("parsing `herdr tab create`: %w", err)
	}
	if response.Result.Tab.TabID == "" || response.Result.RootPane.PaneID == "" {
		return "", "", fmt.Errorf("`herdr tab create` returned no tab or root pane")
	}
	return response.Result.Tab.TabID, response.Result.RootPane.PaneID, nil
}

func (c *client) splitPane(paneID, direction string, ratio float64, cwd string) (string, error) {
	args := []string{
		"pane", "split", paneID,
		"--direction", direction,
		"--ratio", strconv.FormatFloat(ratio, 'f', -1, 64),
		"--cwd", cwd,
		"--no-focus",
	}
	output, err := c.run(args...)
	if err != nil {
		return "", err
	}
	var response struct {
		Result struct {
			Pane paneInfo `json:"pane"`
		} `json:"result"`
	}
	if err := decode(output, &response); err != nil {
		return "", fmt.Errorf("parsing `herdr pane split`: %w", err)
	}
	if response.Result.Pane.PaneID == "" {
		return "", fmt.Errorf("`herdr pane split` returned no pane")
	}
	return response.Result.Pane.PaneID, nil
}

func (c *client) movePane(sourcePane, targetTab, placeholder string, focus bool) (string, error) {
	args := []string{
		"pane", "move", sourcePane,
		"--tab", targetTab,
		"--target-pane", placeholder,
		"--split", "right",
		"--ratio", "0.5",
	}
	if focus {
		args = append(args, "--focus")
	} else {
		args = append(args, "--no-focus")
	}
	output, err := c.run(args...)
	if err != nil {
		return "", err
	}
	var response struct {
		Result struct {
			Move struct {
				Changed bool     `json:"changed"`
				Reason  string   `json:"reason"`
				Pane    paneInfo `json:"pane"`
			} `json:"move_result"`
		} `json:"result"`
	}
	if err := decode(output, &response); err != nil {
		return "", fmt.Errorf("parsing `herdr pane move`: %w", err)
	}
	if !response.Result.Move.Changed {
		return "", fmt.Errorf("Herdr did not move pane %s (%s)", sourcePane, response.Result.Move.Reason)
	}
	if response.Result.Move.Pane.PaneID == "" {
		return "", fmt.Errorf("`herdr pane move` returned no pane")
	}
	return response.Result.Move.Pane.PaneID, nil
}

func (c *client) closePane(paneID string) error {
	_, err := c.run("pane", "close", paneID)
	return err
}

func (c *client) closeTab(tabID string) error {
	_, err := c.run("tab", "close", tabID)
	return err
}

func (c *client) focusTab(tabID string) error {
	_, err := c.run("tab", "focus", tabID)
	return err
}

func (c *client) zoomOff(paneID string) error {
	_, err := c.run("pane", "zoom", "--pane", paneID, "--off")
	return err
}

func (c *client) cleanupTarget(tabID string, failure error) error {
	if err := c.closeTab(tabID); err != nil {
		return fmt.Errorf("%w; cleanup also failed: %v", failure, err)
	}
	return failure
}

func orderedPanes(panes []layoutPane) []layoutPane {
	ordered := append([]layoutPane(nil), panes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Rect.Y != ordered[j].Rect.Y {
			return ordered[i].Rect.Y < ordered[j].Rect.Y
		}
		if ordered[i].Rect.X != ordered[j].Rect.X {
			return ordered[i].Rect.X < ordered[j].Rect.X
		}
		return ordered[i].PaneID < ordered[j].PaneID
	})
	return ordered
}

func containsPane(panes []layoutPane, paneID string) bool {
	for _, pane := range panes {
		if pane.PaneID == paneID {
			return true
		}
	}
	return false
}

func decode(output []byte, value any) error {
	if err := json.Unmarshal(output, value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
