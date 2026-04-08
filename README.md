<div align="center">

# 🪟 layouts

**Tmux layout manager — predefined pane arrangements from a single config.**

*One command. Windows, panes, splits, and commands.*

</div>

Define tmux layouts once in YAML, apply them to any session. Like tmuxinator's layout system, but standalone and minimal — just the layout part, nothing else.

- **Declarative layouts** — windows, panes, split directions, sizes, and startup commands in YAML
- **Apply anywhere** — works on any tmux session, not tied to a specific project
- **fzf picker** — pick a layout interactively when you don't specify one
- **Session creation** — create new tmux sessions with layouts pre-applied
- **Grid rearrange** — reshape a messy window into a clean `cols × rows` grid without losing pane content
- **Zero state** — no database, no state files, just config

---

## Install

Requires Go 1.21+ and [fzf](https://github.com/junegunn/fzf).

```sh
git clone <repo-url> layouts
cd layouts
make install    # builds and copies to $GOPATH/bin
```

**Fish shell helper** — install the `ly` shorthand function:

```sh
make fish       # copies ly.fish to ~/.config/fish/functions/
```

## Quick Start

```sh
# 1. Create config with example layouts
layouts init

# 2. See what's available
layouts list

# 3. Apply a layout to your current tmux session
layouts apply dev
```

## Config

Location: `~/.config/layouts/config.yaml` (created by `layouts init`)

```yaml
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
      - name: test
        panes:
          - name: test-1
            size: "50%"
          - name: test-2
            size: "50%"

  simple:
    windows:
      - name: main
        panes:
          - name: editor
            size: "70%"
            cmd: nvim .
          - name: shell
```

Each layout has one or more **windows**, each with **panes**:

| Field | Description |
|-------|-------------|
| `name` | Window or pane name |
| `split` | `horizontal` (side by side, default) or `vertical` (stacked) |
| `size` | Percentage of the window (e.g. `70%`). Unspecified panes split remaining space equally |
| `cmd` | Command to run in the pane. Empty = shell prompt |

Top-level optional fields:

| Field | Description |
|-------|-------------|
| `default` | Layout name to use when none is specified |
| `editor` | Editor for `layouts config` (falls back to `$EDITOR`, then `nvim`) |

## Commands

```sh
layouts apply              # pick layout via fzf, apply to current session
layouts apply dev          # apply named layout
layouts apply dev -d .     # apply using specific working directory

layouts list               # list all layouts with window/pane counts
layouts show dev           # show layout tree with panes, sizes, commands

layouts new mysession dev  # create new tmux session with layout
layouts new mysession      # create session with default layout (if set)

layouts grid 4x2           # rearrange current window's panes into 4x2 grid
layouts grid 3x3           # 3x3 grid (creates empty panes if needed)

layouts config             # open config in editor
layouts config --path      # print config file path
layouts init               # create config with example layouts

layouts --version          # print version
```

Most commands have short aliases: `apply`→`a`, `list`→`ls`/`l`, `show`→`s`, `new`→`n`, `grid`→`g`, `config`→`c`/`cfg`.

## Show Output

`layouts show dev` renders a tree view:

```
dev

  window 1: claude
    split: horizontal
    ├ claude [25%] → claude --dangerously-skip-permissions
    ├ editor [50%] → nvim .
    └ shell [25%]

  window 2: test
    split: horizontal
    ├ test-1 [50%]
    └ test-2 [50%]
```

## Fish Alias

The `ly` function maps subcommands to `layouts`:

```sh
ly              # layouts list
ly a dev        # layouts apply dev
ly s dev        # layouts show dev
ly n work dev   # layouts new work dev
ly g 4x2        # layouts grid 4x2
ly c            # layouts config
```

## How It Works

`layouts apply` adds new windows to your **current** tmux session. It does not touch existing windows — it only creates new ones. Each window is split according to the layout spec, and pane commands are sent via `tmux send-keys`.

`layouts new` creates a **new** tmux session with the layout pre-applied. The first window reuses the session's initial window (renamed), subsequent windows are created fresh.

Pane sizes are computed proportionally. If some panes have explicit sizes and others don't, the remaining space is divided equally among unspecified panes. Sizes must sum to at most 100%.

## Grid Rearrange

`layouts grid <cols>x<rows>` reshapes the **current** window into a clean `cols × rows` grid. Format is `colsxrows` (width × height), so `4x2` means 4 wide, 2 tall — 8 panes total.

```sh
layouts grid 4x2   # 4 cols × 2 rows
layouts grid 3x3   # 3×3 grid
layouts grid 2x1   # two panes side by side
```

- **Content-preserving** — existing panes keep their running commands and scrollback. Grid uses `break-pane` + `join-pane` under the hood, not kill/split.
- **Fills gaps** — if the window has fewer panes than the grid needs, empty panes are created to fill it out.
- **Refuses to destroy** — if the window already has *more* panes than the grid holds, grid errors out. Close the extras yourself first.
- **Even spacing** — rows and columns are resized to equal `100/rows%` and `100/cols%`.

## Integration with Grove

If you use [grove](https://github.com/...) for worktree management, you can reference layouts by name in grove's repo config:

```yaml
# ~/.config/grove/config.yaml
repos:
  - path: ~/code/myproject
    layout: dev    # references a layout defined in layouts config
```

---

> Personal tool built for my own workflow. Feel free to fork and adapt.
