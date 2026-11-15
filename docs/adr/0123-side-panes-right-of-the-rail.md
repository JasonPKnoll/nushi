# 0123. Side panes right of the details rail, which tucks itself away

Status: accepted (the owner, 2026-11-15).

## Context
A chat was laid out transcript | side panes | details rail, the chat's header across all of it, and the side panes (the side chat, Files, a subagent's or a command's pane, the preview: a file or a browser tab) lay over the transcript once it got too narrow, the rail staying put. The owner, with screenshots of Claude's desktop app (`nushi-review/2026-11-15-right-panes/owner/`): "I want the browser and file viewer to open to the right of the widget rail like claude desktop. I also want it to auto-hide when it would make the chat too small." Claude's header ends at its rail's right edge and its browser's tabs are in the window's top row; on moving the header's buttons for that: "I think maybe everything should? If we move the buttons then we unlock more realastate for other things."

## Decision
- **Order: transcript | details rail | every side pane**, the panes in their old order among themselves (side chat, Files, subagent or command, preview).
- **The chat's header spans the transcript and the rail; each pane runs the pane's full height**, its own top row `PaneHeader.height` high, its hairline level with the header's. The browser's and the file preview's tabs were already in that row, which is now the window's top row (ADR 0070); each pane keeps its × and its buttons there, and its address bar or path line under it. The terminal runs under the transcript and the rail.
- **Over the transcript, as before.** Narrower than the beside line, where the panes lie over the transcript, the header and the terminal run across the whole chat pane and the panes lie between them, under the header: their top rows sit under it, the preview's tabs aren't in the window's top row, and the header's buttons stay at its right end.
- **The header's buttons move with the panes**: opening a pane puts its left edge where the buttons ended, and they glide left by its width, in step with its slide. This is a deliberate exception to "a toggle never moves when pressed" (ADR 0111): the opened pane takes their place. The details' and the terminal's buttons still never move them.
- **The rail tucks itself away for the panes' room, then the panes give width.** One pure function (`besideFit`, `beside_fit.dart`) shares the room. The rail goes first, when the transcript would keep less than 484 beside it (360 beside a subagent's pane or the preview). Then, where the transcript would still keep less than its least beside the panes alone (556 beside the side chat or Files, 360 beside those wide panes), the panes narrow toward their own least widths, live and never saved: the widest gives first (the preview or a subagent's pane, as a rule), down to its least, then the next, the transcript keeping its least. Their saved and dragged widths stay as the user left them, and are back as soon as there's room. Only once every pane is at its least do they lie over the transcript. The transcript's least widths are the old ones (ADR 0092): the new order changes what gives way first, not how narrow the transcript gets. With no pane the rail shows from a chat area of 804, as before.
  - It's decided on widths alone, the rail an input to nothing: whether the panes lie over is read from their least widths alone, the rail and the narrowing from the widths as dragged. A pane's edge dragged while the panes are beside the transcript stops where it keeps its least, the rest as they are (narrowed, a pane can be dragged narrower, which gives the rest their width back, but no wider), so a drag can't move the panes over it under the pointer; the rail tucks and comes back as the edge moves. Lying over it, a pane is at its least, and its edge stops there (an uncapped pane went on to 48 before). A pane being dragged gives width last. Widths are whole points while dragged.
  - It's never saved: `showDetails` and `app.json` keep the user's choice, the header's button is lit by it ("Hide details (tucked away while the panes need the room)"), a press changes only the choice, and Artifacts… lists in the palette while there's no room. Tucked, the rail stays built (not drawn, not reached, its tickers off), so it comes back as it was and asks the daemon nothing again; it goes on asking what fills the context after each turn (at most once a minute), as a shown one does, so the Context widget is current when it's back; the focus in it goes to the chat's keys. It slides as the panes do; Reduce Motion jumps.
- **Expand covers the whole chat pane** (ADR 0094), its header and terminal too; its tabs start past the traffic lights with the sidebar hidden. ⌃` while expanded puts it back and shows the terminal in one press (one already open under the cover is shown, not closed); the chat's terminal opening any other way puts it back too.
- **In a chat narrower than the panes' least widths, they narrow alike.** Over the transcript a pane is never narrower than its least, so in a chat area narrower than the open panes' least widths (the side chat, Files and the preview need 800) the leftmost would run past its left edge and be cut. They are narrowed instead, each by the same share, to fit from its left edge (`Beside`); wider, they're back at their least or more. Leaving the leftmost out was tried and dropped: Files would vanish as a file opened from it in a 720 window.
- An agent's `show` opens the preview where it fits with the rail tucked and the panes narrowed to their least (ADR 0068's "never over the box" holds).
- The phone is untouched; Linux has the same layout.

## Consequences
- Between the old line where the panes lay over the transcript and the new one, they stay beside it, the rail tucked away, and narrow below the middle column (the table below; A is the chat's pane, the window less 268 with the sidebar, 16 without). Above the first column the rail shows; under the last, the panes lie over the transcript, each at its least (side chat 300, Files 200, subagent or command 220, preview 300).

  | Open (default widths) | Rail from A | Narrowing under A | Beside from A |
  |---|---|---|---|
  | none | 804 | — | — |
  | side chat 380 | 1184 | 936 | 856 |
  | Files 260 | 1064 | 816 | 756 |
  | side chat + Files | 1444 | 1196 | 1056 |
  | subagent or command 264 | 944 | 624 | 580 |
  | preview 560 | 1240 | 920 | 660 |
  | Files + preview | 1500 | 1180 | 860 |
  | side chat + Files + preview | 1880 | 1560 | 1160 |

- A header 360 wide happens now. Past the Jira key and the GitHub chips, which fold into ⋯, the agent's mark folds (the state's dot's tooltip names the agent), then the tabs' field, into ⋯ as the tabs, so the title keeps its 80 even with the sidebar hidden on a Mac, where the traffic lights take the row's start.
- A header with no pane open changed too, as the owner accepted: it gives its title the room the row leaves and measures the agent's name (f8650d40), where it had capped both from worst-case widths, so the header's row in the `spacing_turn_720`, `header_menu`, `reopen` and `reopen_gone` goldens moved.
- The notices still float centred on the whole pane, so they can lie over a pane's address row; centring them on the chat is a follow-up.
- The Changes and Pull request tabs have no panes, as before; the header and the terminal stay the same widgets from tab to tab.

## Alternatives considered
- Keeping the rail at the right with the panes left of it: not what the owner asked; Claude's rail is beside the chat.
- Panes lying over the transcript at their saved widths as soon as it would keep less than its least (this ADR as first accepted): between that line and the one where they're all at their least, a pane covered the transcript that could have given it room, and at the line the window's layout jumped.
- Narrowing the transcript's least beside every pane to 484: the rail would hide where it showed before beside a preview.
- Saving the hide (turning `showDetails` off): the panel would stay away once there's room again, and the user's choice would be the window's.
- Panes under the header at every width, as before: the header's buttons couldn't then end where Claude's do, and every pane lost a row's height.
