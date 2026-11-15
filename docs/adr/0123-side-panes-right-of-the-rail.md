# 0123. Side panes right of the details rail, which tucks itself away

Status: accepted (the owner, 2026-11-15).

## Context
A chat was laid out transcript | side panes | details rail, the chat's header across all of it, and the side panes (the side chat, Files, a subagent's or a command's pane, the preview: a file or a browser tab) lay over the transcript once it got too narrow, the rail staying put. The owner, with screenshots of Claude's desktop app (`nushi-review/2026-11-15-right-panes/owner/`): "I want the browser and file viewer to open to the right of the widget rail like claude desktop. I also want it to auto-hide when it would make the chat too small." Claude's header ends at its rail's right edge and its browser's tabs are in the window's top row; on moving the header's buttons for that: "I think maybe everything should? If we move the buttons then we unlock more realastate for other things."

## Decision
- **Order: transcript | details rail | every side pane**, the panes in their old order among themselves (side chat, Files, subagent or command, preview).
- **The chat's header spans the transcript and the rail; each pane runs the pane's full height**, its own top row `PaneHeader.height` high, its hairline level with the header's. The browser's and the file preview's tabs were already in that row, which is now the window's top row (ADR 0070); each pane keeps its × and its buttons there, and its address bar or path line under it. The terminal runs under the transcript and the rail.
- **The header's buttons move with the panes**: opening a pane puts its left edge where the buttons ended, and they glide left by its width, in step with its slide. This is a deliberate exception to "a toggle never moves when pressed" (ADR 0111): the opened pane takes their place. The details' and the terminal's buttons still never move them.
- **The rail tucks itself away for the panes' room.** One pure function (`besideFit`, `beside_fit.dart`) shares the room: the panes lie over the transcript only when it would keep less than its least beside them alone (556 beside the side chat or Files, 360 beside a subagent's pane or the preview); before that, the rail goes when the transcript would keep less than 484 beside it (360 beside those wide panes). The numbers are the old ones (ADR 0092): the new order changes what gives way first, not how narrow the transcript gets. With no pane the rail shows from a chat area of 804, as before.
  - It's decided on widths alone, the rail an input to nothing: whether the panes lie over is read from their saved widths, the rail from the widths as dragged. A pane's edge dragged while the panes are beside the transcript stops where it keeps its least (an uncapped pane went on to 48 before; over the transcript it still does), so a drag can't move the panes over it under the pointer; the rail tucks and comes back as the edge moves. Widths are whole points while dragged.
  - It's never saved: `showDetails` and `app.json` keep the user's choice, the header's button is lit by it ("Hide details (tucked away while the panes need the room)"), a press changes only the choice, and Artifacts… lists in the palette while there's no room. Tucked, the rail stays built (not drawn, not reached, its tickers off), so it comes back as it was and asks the daemon nothing again; the focus in it goes to the chat's keys. It slides as the panes do; Reduce Motion jumps.
- **Expand covers the whole chat pane** (ADR 0094), its header and terminal too; its tabs start past the traffic lights with the sidebar hidden. ⌃` while expanded puts it back and shows the terminal in one press (one already open under the cover is shown, not closed); the chat's terminal opening any other way puts it back too.
- An agent's `show` opens the preview where it fits with the rail tucked (ADR 0068's "never over the box" holds).
- The phone is untouched; Linux has the same layout.

## Consequences
- Between the old line where the panes lay over the transcript and the new one, they stay beside it and the rail tucks away (the table below; A is the chat's pane, the window less 268 with the sidebar, 16 without). Above the first column the rail shows.

  | Open (default widths) | Rail from A | Beside from A |
  |---|---|---|
  | none | 804 | — |
  | side chat 380 | 1184 | 936 |
  | Files 260 | 1064 | 816 |
  | side chat + Files | 1444 | 1196 |
  | subagent or command 264 | 944 | 624 |
  | preview 560 | 1240 | 920 |
  | Files + preview | 1500 | 1180 |
  | side chat + Files + preview | 1880 | 1560 |

- A header 360 wide happens now: the title is cut shorter than its old 80 when even its folds leave less, and last of all the Jira key and the GitHub chips fold into ⋯.
- The notices still float centred on the whole pane, so they can lie over a pane's address row; centring them on the chat is a follow-up.
- The Changes and Pull request tabs have no panes, as before; the header and the terminal stay the same widgets from tab to tab.

## Alternatives considered
- Keeping the rail at the right with the panes left of it: not what the owner asked; Claude's rail is beside the chat.
- Narrowing the transcript's least beside every pane to 484: the rail would hide where it showed before beside a preview.
- Saving the hide (turning `showDetails` off): the panel would stay away once there's room again, and the user's choice would be the window's.
- Panes under the header, as before: the header's buttons couldn't then end where Claude's do, and every pane lost a row's height.
