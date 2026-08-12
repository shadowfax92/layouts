package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"layouts/internal/herdr"
	"layouts/internal/tmux"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(gridCmd)
}

type gridBackend int

const (
	gridBackendNone gridBackend = iota
	gridBackendHerdr
	gridBackendTmux
)

var gridCmd = &cobra.Command{
	Use:         "grid <cols>x<rows>",
	Aliases:     []string{"g"},
	Annotations: map[string]string{"group": "Layouts:"},
	Short:       "Arrange the current tmux window or Herdr tab into a grid",
	Long: `Rearrange the panes in the current tmux window or Herdr tab into a cols × rows grid.

Existing pane content is preserved. If the window or tab has fewer panes than
the grid requires, empty panes are created. If it has more, grid refuses to
run rather than killing panes.

  layouts grid 4x2   — 4 columns, 2 rows (8 panes total)
  layouts grid 3x3   — 3x3 grid (9 panes)
  layouts grid 2x1   — two side-by-side panes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cols, rows, err := parseGridSpec(args[0])
		if err != nil {
			return err
		}

		var before, created int
		var surface string
		switch detectGridBackend() {
		case gridBackendHerdr:
			surface = "tab"
			before, created, err = herdr.ArrangeGrid(cols, rows)
		case gridBackendTmux:
			surface = "window"
			before, created, err = tmux.ArrangeGrid(cols, rows)
		default:
			return fmt.Errorf("must be inside a tmux or Herdr session")
		}
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("Arranged %s into %dx%d grid (%d panes", surface, cols, rows, cols*rows)
		if created > 0 {
			msg += fmt.Sprintf(", created %d empty", created)
		} else if before == cols*rows {
			msg += ", rearranged in place"
		}
		msg += ")"
		fmt.Println(msg)
		return nil
	},
}

func detectGridBackend() gridBackend {
	switch {
	// Herdr panes can inherit TMUX from the process that launched Herdr.
	case herdr.IsInsideHerdr():
		return gridBackendHerdr
	case tmux.IsInsideTmux():
		return gridBackendTmux
	default:
		return gridBackendNone
	}
}

func parseGridSpec(s string) (int, int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid grid %q — expected format like 4x2 (cols x rows)", s)
	}
	cols, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || cols < 1 {
		return 0, 0, fmt.Errorf("invalid cols in %q", s)
	}
	rows, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || rows < 1 {
		return 0, 0, fmt.Errorf("invalid rows in %q", s)
	}
	return cols, rows, nil
}
