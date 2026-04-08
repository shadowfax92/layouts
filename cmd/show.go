package cmd

import (
	"fmt"

	"layouts/internal/config"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showCmd)
}

var (
	winColor  = color.New(color.FgCyan, color.Bold)
	paneColor = color.New(color.FgHiWhite)
	cmdColor  = color.New(color.FgYellow)
	sizeColor = color.New(color.Faint)
)

var showCmd = &cobra.Command{
	Use:         "show <name>",
	Aliases:     []string{"s"},
	Annotations: map[string]string{"group": "Layouts:"},
	Short:       "Show layout details",
	Args:        cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}

		layout := cfg.FindLayout(args[0])
		if layout == nil {
			return fmt.Errorf("layout %q not found", args[0])
		}

		fmt.Printf("%s\n\n", nameColor.Sprint(args[0]))
		printLayout(layout)

		return nil
	},
}

func printLayout(layout *config.LayoutConfig) {
	for i, win := range layout.Windows {
		split := "horizontal"
		if win.Split != "" {
			split = win.Split
		}
		fmt.Printf("  %s %s\n", winColor.Sprintf("window %d:", i+1), winColor.Sprint(win.Name))
		fmt.Printf("    split: %s\n", split)
		if win.Rows > 1 {
			cols := len(win.Panes) / win.Rows
			fmt.Printf("    grid: %dx%d\n", cols, win.Rows)
		} else if win.Layout != "" {
			fmt.Printf("    layout: %s\n", win.Layout)
		}

		for j, pane := range win.Panes {
			name := pane.Name
			if name == "" {
				name = fmt.Sprintf("pane-%d", j+1)
			}

			connector := "├"
			if j == len(win.Panes)-1 {
				connector = "└"
			}

			line := fmt.Sprintf("    %s %s", paneColor.Sprint(connector), paneColor.Sprint(name))
			if pane.Size != "" {
				line += " " + sizeColor.Sprintf("[%s]", pane.Size)
			}
			if pane.Cmd != "" {
				line += " " + cmdColor.Sprintf("→ %s", pane.Cmd)
			}
			fmt.Println(line)
		}

		if i < len(layout.Windows)-1 {
			fmt.Println()
		}
	}
}
