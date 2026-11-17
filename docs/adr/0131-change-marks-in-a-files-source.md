# 0131. Change marks in a file's source

Status: accepted, 2026-11-17.

## Context
The owner: "I see the green dot and want to know what changed. I don't think we need a diff but just to know which lines changed." The tree's dot says only that the chat wrote a file; to see what changed meant opening Changes and finding the file.

## Decision
- **Change marks against the last commit** in the Files pane's source, read-only (`SourceView`) and in the editor (`SourceEditor`): a solid 3-point bar beside added (green, `done`) and changed (blue, `accentInk`) lines and a wedge (`textSecondary`) on the edge where lines were removed, in the 14 points between a line's number and its text, so the code doesn't move. The base is HEAD of the repository that holds the file, as the Changes tab's Uncommitted has it. The diff's own inks, solid: its ribbon tokens are pale tints that all but vanish as a bar this thin.
- **Hover (after a moment) or click a mark** for a card of what the last commit had there: "Before" over a changed run's old lines, "N lines removed" over a removed run's, "New lines" for an added run; read-only, selectable, 40 lines at most. Not a diff view. Near the window's foot it opens over the line.
- **The base comes from GetDiff with a path** (API 121, additive): the file's own folder and name, so a second root or a nested repository is compared against its own HEAD, and only that file is read. The app diffs it against the text it shows (`TextDiff`, as the diff view does), so the marks follow unsaved edits after a pause in typing and no stale-answer protocol is needed. It asks again on each new version from the file's watch, at a turn's end, on a commit made in the app and when the chat's folder changes.
- **No marks** where they'd mislead or be noise: a file new to git (untracked, or added; its path line says **New file** instead), an ignored file, a link, a binary or over 1 MiB, a file outside any repository, more than 10,000 lines, JSON the view pretty-printed, and a rendered Markdown, CSV, HTML, SVG or Mermaid view (their Source view has them). An older daemon: none.
- **Show changes**: a file with the tree's dot has a right-click menu with Show changes, which opens it in the chat's Changes tab. The dot stays what it was (files the chat wrote), so after a commit a file can have the dot and no marks.
- No settings, no new pane, nothing on the phone.

## Consequences
- A commit made outside Nushi shows on the next file change, turn end, commit in the app or tab opened, not at once.
- The marks are Myers' line diff, as the app's diff view draws it; a user's `diff.algorithm` can place a hunk's edge differently in `git diff`.
- A change of line endings only, or of the last line break only, shows no marks (Changes still lists the file).

## Alternatives considered
- A new RPC returning hunks from `git diff`: a hunk parser and a sha check before and after, and it would still not know the editor's unsaved text.
- GetDiff of the whole folder: every changed file's text (up to 32 MiB) for one file's marks, and the file could come back skipped past the view's cap.
- Hiding the marks while there are unsaved edits: diffing the shown text keeps them true instead.
