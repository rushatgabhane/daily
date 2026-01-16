# daily

A sleek TUI for submitting daily reports.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Wizard-style interface** - One field at a time for small terminals
- Saves form URL after first use
- Sets today's date by default
- Fetches issue titles automatically using `gh` CLI (invisible to user)
- Shows completed fields as you progress
- Form validation

## Prerequisites

- Go 1.21 or later
- `gh` CLI (GitHub CLI) - for auto-fetching issue titles
  ```bash
  brew install gh
  gh auth login
  ```

## Installation

```bash
# Build and install
make install

# Or build locally
make build
```

## Usage

First time, you'll be asked for your Google Forms URL:

```bash
daily
```

The form URL will be saved to `~/.config/daily/config.json` and reused automatically.

### How it works

The tool presents fields one at a time in wizard-style:

1. **Date** - Defaults to today (DD/MM/YYYY format)
2. **Issue Link** - GitHub issue URL (fetches title automatically)
3. **Project** - Defaults to "N/A"
4. **Progress Note** - Simple text input for your update
5. **Hours Spent** - Number between 0-12
6. **Open** - Opens pre-filled form in browser

As you complete each field, it's shown in the "Completed" section above.

### Navigation

- `enter` - Move to next field
- `ctrl+c` - Quit anytime

## Development

```bash
# Run without installing
make run

# Build binary
make build

# Clean build artifacts
make clean
```

## License

MIT
