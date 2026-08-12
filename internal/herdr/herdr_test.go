package herdr

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestArrangeGridMovesLivePanesBackIntoOriginalTab(t *testing.T) {
	script := &scriptedRunner{
		t: t,
		calls: []scriptedCall{
			{
				args:   []string{"pane", "layout", "--current"},
				output: `{"result":{"layout":{"workspace_id":"w1","tab_id":"w1:t1","zoomed":false,"focused_pane_id":"w1:p2","panes":[{"pane_id":"w1:p3","rect":{"x":0,"y":20,"width":50,"height":20}},{"pane_id":"w1:p2","rect":{"x":50,"y":0,"width":50,"height":40}},{"pane_id":"w1:p1","rect":{"x":0,"y":0,"width":50,"height":20}}]}}}`,
			},
			{
				args:   []string{"pane", "get", "w1:p2"},
				output: `{"result":{"pane":{"pane_id":"w1:p2","cwd":"/repo","foreground_cwd":"/repo"}}}`,
			},
			{
				args:   []string{"tab", "get", "w1:t1"},
				output: `{"result":{"tab":{"tab_id":"w1:t1","label":"dev","focused":true}}}`,
			},
			{
				args:   []string{"tab", "create", "--workspace", "w1", "--cwd", "/repo", "--label", "layouts-grid-holding", "--no-focus"},
				output: `{"result":{"root_pane":{"pane_id":"w1:p4"},"tab":{"tab_id":"w1:t2"}}}`,
			},
			{
				args:   []string{"pane", "move", "w1:p2", "--tab", "w1:t2", "--target-pane", "w1:p4", "--split", "right", "--ratio", "0.5", "--no-focus"},
				output: `{"result":{"move_result":{"changed":true,"pane":{"pane_id":"w1:p2"}}}}`,
			},
			{
				args:   []string{"pane", "move", "w1:p3", "--tab", "w1:t2", "--target-pane", "w1:p4", "--split", "right", "--ratio", "0.5", "--no-focus"},
				output: `{"result":{"move_result":{"changed":true,"pane":{"pane_id":"w1:p3"}}}}`,
			},
			{
				args:   []string{"pane", "split", "w1:p1", "--direction", "down", "--ratio", "0.5", "--cwd", "/repo", "--no-focus"},
				output: `{"result":{"pane":{"pane_id":"w1:p5"}}}`,
			},
			{
				args:   []string{"pane", "split", "w1:p1", "--direction", "right", "--ratio", "0.5", "--cwd", "/repo", "--no-focus"},
				output: `{"result":{"pane":{"pane_id":"w1:p6"}}}`,
			},
			{
				args:   []string{"pane", "split", "w1:p5", "--direction", "right", "--ratio", "0.5", "--cwd", "/repo", "--no-focus"},
				output: `{"result":{"pane":{"pane_id":"w1:p7"}}}`,
			},
			{
				args:   []string{"pane", "move", "w1:p2", "--tab", "w1:t1", "--target-pane", "w1:p6", "--split", "right", "--ratio", "0.5", "--focus"},
				output: `{"result":{"move_result":{"changed":true,"pane":{"pane_id":"w1:p2"}}}}`,
			},
			{args: []string{"pane", "close", "w1:p6"}, output: `{}`},
			{
				args:   []string{"pane", "move", "w1:p3", "--tab", "w1:t1", "--target-pane", "w1:p5", "--split", "right", "--ratio", "0.5", "--no-focus"},
				output: `{"result":{"move_result":{"changed":true,"pane":{"pane_id":"w1:p3"}}}}`,
			},
			{args: []string{"pane", "close", "w1:p5"}, output: `{}`},
			{args: []string{"tab", "close", "w1:t2"}, output: `{}`},
			{args: []string{"tab", "focus", "w1:t1"}, output: `{}`},
		},
	}

	before, created, err := (&client{run: script.run}).arrangeGrid(2, 2)
	if err != nil {
		t.Fatalf("arrangeGrid returned error: %v", err)
	}
	if before != 3 || created != 1 {
		t.Fatalf("counts = (%d, %d), want (3, 1)", before, created)
	}
	script.assertDone()
}

func TestArrangeGridRejectsTooManyPanesBeforeMutation(t *testing.T) {
	script := &scriptedRunner{
		t: t,
		calls: []scriptedCall{{
			args:   []string{"pane", "layout", "--current"},
			output: `{"result":{"layout":{"workspace_id":"w1","tab_id":"w1:t1","focused_pane_id":"w1:p1","panes":[{"pane_id":"w1:p1"},{"pane_id":"w1:p2"},{"pane_id":"w1:p3"}]}}}`,
		}},
	}

	before, created, err := (&client{run: script.run}).arrangeGrid(2, 1)
	if err == nil || !strings.Contains(err.Error(), "holds only 2") {
		t.Fatalf("error = %v, want capacity error", err)
	}
	if before != 3 || created != 0 {
		t.Fatalf("counts = (%d, %d), want (3, 0)", before, created)
	}
	script.assertDone()
}

func TestSplitEvenUsesValidBalancedRatios(t *testing.T) {
	nextID := 1
	var ratios []float64
	run := func(args ...string) ([]byte, error) {
		for i, arg := range args {
			if arg != "--ratio" {
				continue
			}
			ratio, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				return nil, err
			}
			ratios = append(ratios, ratio)
		}
		id := fmt.Sprintf("p%d", nextID)
		nextID++
		return []byte(fmt.Sprintf(`{"result":{"pane":{"pane_id":%q}}}`, id)), nil
	}

	panes, err := (&client{run: run}).splitEven("root", 12, "right", "/repo")
	if err != nil {
		t.Fatalf("splitEven returned error: %v", err)
	}
	if len(panes) != 12 {
		t.Fatalf("pane count = %d, want 12", len(panes))
	}
	for _, ratio := range ratios {
		if ratio < 0.1 || ratio > 0.9 {
			t.Fatalf("ratio %v falls outside Herdr's supported range", ratio)
		}
	}
}

type scriptedCall struct {
	args   []string
	output string
}

type scriptedRunner struct {
	t     *testing.T
	calls []scriptedCall
	next  int
}

func (s *scriptedRunner) run(args ...string) ([]byte, error) {
	s.t.Helper()
	if s.next >= len(s.calls) {
		s.t.Fatalf("unexpected Herdr call: %q", args)
	}
	call := s.calls[s.next]
	s.next++
	if !reflect.DeepEqual(args, call.args) {
		s.t.Fatalf("Herdr call %d = %q, want %q", s.next, args, call.args)
	}
	return []byte(call.output), nil
}

func (s *scriptedRunner) assertDone() {
	s.t.Helper()
	if s.next != len(s.calls) {
		s.t.Fatalf("used %d Herdr calls, want %d", s.next, len(s.calls))
	}
}
