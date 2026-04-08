package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"layouts/internal/tmux"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(gridCmd)
}

var gridCmd = &cobra.Command{
	Use:         "grid <cols>x<rows>",
	Aliases:     []string{"g"},
	Annotations: map[string]string{"group": "Layouts:"},
	Short:       "Arrange current window's panes into a grid",
	Long: `Rearrange the panes in the current tmux window into a cols × rows grid.

Existing pane content is preserved. If the window has fewer panes than the
grid requires, empty panes are created. If it has more, grid refuses to run
rather than killing panes.

  layouts grid 4x2   — 4 columns, 2 rows (8 panes total)
  layouts grid 3x3   — 3x3 grid (9 panes)
  layouts grid 2x1   — two side-by-side panes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !tmux.IsInsideTmux() {
			return fmt.Errorf("must be inside a tmux session")
		}

		cols, rows, err := parseGridSpec(args[0])
		if err != nil {
			return err
		}

		before, created, err := tmux.ArrangeGrid(cols, rows)
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("Arranged window into %dx%d grid (%d panes", cols, rows, cols*rows)
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
