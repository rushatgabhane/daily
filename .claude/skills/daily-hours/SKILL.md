---
name: daily-hours
description: Log daily hours from GitHub activity using the `daily` CLI tool. Use when the user asks to log hours, create daily entries, or fill timesheets.
---

# Daily Hours Logger

Log work hours by searching GitHub activity and submitting via the `daily` CLI tool.

## Steps

### 1. Get the date range from the user

### 2. Search GitHub activity for those dates

Determine the user's GitHub handle via `gh api user --jq '.login'`. Then query across the user's org repos:

- PRs authored, commented on, or reviewed
- Issues assigned, commented on, or closed
- Commits in local repos

Use `gh search prs`, `gh search issues`, `gh api search/issues`, and `git log` with the user's handle.

### 3. Organize into daily entries

- Target **6-8 hours per day, 30-40 hours per week**
- Group related work under parent tracking issues where possible
- Each entry: issue/PR URL, short progress note, hours

### 4. Show summary first

Before submitting, show planned entries so the user can adjust:

```
March 15 (3 entries, 8h total)
- 3h | A11y tracking | Reviewed contributor PRs, closed 5 issues
- 3h | Change Approver | Addressed review feedback on Auth PR
- 2h | Export PDF | S3 upload investigation
```

### 5. Submit one at a time

```bash
daily --date "DD/MM/YYYY" --issue "URL" --progress "description" --hours N
```

Then `open` the generated URL. **Wait for user to say "next" before each subsequent entry.**

## Progress note style

- Keep it concise and natural - describe what happened, not a list of PR numbers
- Good: "Reviewed contributor PRs, closed 5 color contrast issues"
- Good: "Created Auth and Web-E PRs for PDF export feature"
- Bad: "Reviewed PRs #85221, #85474, #82673. Closed #76958, #77337, #74866"
- Bad: "Auth PR #20287 review iteration. Created follow-up hardening issue #618159"

Date format for the `daily` tool is **DD/MM/YYYY** (day first).
