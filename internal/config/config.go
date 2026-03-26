package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Default string                  `yaml:"default,omitempty"`
	Editor  string                  `yaml:"editor,omitempty"`
	Layouts map[string]LayoutConfig `yaml:"layouts"`
}

type LayoutConfig struct {
	Windows []WindowConfig `yaml:"windows"`
}

type WindowConfig struct {
	Name   string       `yaml:"name"`
	Split  string       `yaml:"split,omitempty"`  // "horizontal" (side by side, default) or "vertical" (stacked)
	Layout string       `yaml:"layout,omitempty"` // tmux layout algorithm applied after pane creation (e.g. "tiled")
	Panes  []PaneConfig `yaml:"panes"`
}

type PaneConfig struct {
	Name string `yaml:"name,omitempty"`
	Cmd  string `yaml:"cmd,omitempty"`
	Size string `yaml:"size,omitempty"` // e.g. "70%"
}

func ConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "layouts", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "layouts", "config.yaml")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no config found — run `layouts init` to create one")
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Layouts == nil {
		c.Layouts = make(map[string]LayoutConfig)
	}
}

func (c *Config) Validate() error {
	for name, layout := range c.Layouts {
		if len(layout.Windows) == 0 {
			return fmt.Errorf("layout %q: must have at least one window", name)
		}
		for _, win := range layout.Windows {
			if win.Name == "" {
				return fmt.Errorf("layout %q: window name is required", name)
			}
			if win.Split != "" && win.Split != "horizontal" && win.Split != "vertical" {
				return fmt.Errorf("layout %q: window %q: split must be \"horizontal\" or \"vertical\"", name, win.Name)
			}
			validLayouts := map[string]bool{"": true, "tiled": true, "even-horizontal": true, "even-vertical": true, "main-horizontal": true, "main-vertical": true}
			if !validLayouts[win.Layout] {
				return fmt.Errorf("layout %q: window %q: layout must be one of: tiled, even-horizontal, even-vertical, main-horizontal, main-vertical", name, win.Name)
			}
			if len(win.Panes) == 0 {
				return fmt.Errorf("layout %q: window %q: must have at least one pane", name, win.Name)
			}
			totalSize := 0
			for j, pane := range win.Panes {
				if pane.Size != "" {
					s := strings.TrimSuffix(strings.TrimSpace(pane.Size), "%")
					n, err := strconv.Atoi(s)
					if err != nil || n < 1 || n > 100 {
						return fmt.Errorf("layout %q: window %q: pane %d: invalid size %q (must be 1-100%%)", name, win.Name, j, pane.Size)
					}
					totalSize += n
				}
			}
			if totalSize > 100 {
				return fmt.Errorf("layout %q: window %q: pane sizes sum to %d%% (max 100%%)", name, win.Name, totalSize)
			}
		}
	}

	if c.Default != "" {
		if _, ok := c.Layouts[c.Default]; !ok {
			return fmt.Errorf("default layout %q not found", c.Default)
		}
	}

	return nil
}

func (c *Config) FindLayout(name string) *LayoutConfig {
	if c.Layouts == nil || name == "" {
		return nil
	}
	if l, ok := c.Layouts[name]; ok {
		return &l
	}
	return nil
}

func (c *Config) LayoutNames() []string {
	names := make([]string, 0, len(c.Layouts))
	for name := range c.Layouts {
		names = append(names, name)
	}
	return names
}

func Init() error {
	p := ConfigPath()
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("config already exists at %s", p)
	}

	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}

	content := `# Layouts configuration
# See: layouts config --path

# default: dev
# editor: nvim

layouts:
  dev:
    windows:
      - name: claude
        split: horizontal
        panes:
          - name: claude
            size: "25%"
            cmd: claude --dangerously-skip-permissions
          - name: editor
            size: "50%"
            cmd: nvim .
          - name: shell
            size: "25%"
      - name: codex
        panes:
          - name: codex
            cmd: codex-yolo
          - name: editor
            size: "50%"
            cmd: nvim .
          - name: shell
            size: "25%"
      - name: test
        panes:
          - name: test-1
            size: "50%"
          - name: test-2
            size: "50%"

  mac:
    windows:
      - name: agent
        split: horizontal
        panes:
          - name: claude
            size: "50%"
            cmd: claude --dangerously-skip-permissions
          - name: codex
            size: "50%"
            cmd: codex-yolo
      - name: code
        panes:
          - name: editor
            cmd: nvim .
      - name: test
        panes:
          - name: test

  simple:
    windows:
      - name: main
        panes:
          - name: editor
            size: "70%"
            cmd: nvim .
          - name: shell
`
	return os.WriteFile(p, []byte(content), 0644)
}
