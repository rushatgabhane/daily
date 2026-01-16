# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build      # Build the binary
make run        # Run without installing
make install    # Build and install to /usr/local/bin
make clean      # Remove build artifacts
make uninstall  # Remove from /usr/local/bin
```

## Architecture

This is a single-file Go TUI application (~650 lines in main.go) built with Bubble Tea. It submits daily reports to Google Forms.

**Key components:**
- `model` - Main form state with 4 text inputs (email, date, issue link, project) plus separate progressInput and hoursInput
- `setupModel` - First-run setup to configure Google Forms URL
- Two tea.Msg types: `issueTitleMsg` (async gh CLI fetch) and `submitResultMsg` (form submission result)

**Flow:**
1. Loads config from `~/.config/daily/config.json`
2. If no config, shows setupModel to get Google Forms URL
3. Shows wizard-style form (one field at a time, focusIndex 0-6)
4. On submit, opens pre-filled Google Form in browser

**External dependencies:**
- `git config user.email` - Auto-fills email field
- `gh issue view` - Fetches GitHub issue titles (requires `gh auth login`)
- `open` command - Opens browser (macOS-specific)

**Date format:** Uses DD/MM/YYYY internally, converts to MM/DD/YYYY for the Google Form API via `convertDateFormat()`.
