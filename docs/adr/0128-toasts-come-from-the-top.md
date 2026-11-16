# 0128. Toasts come from the top

Status: accepted, 2026-11-16. Amends [0061](0061-the-daemon-checks-for-a-newer-release.md) (where the newer release's card sits) and [0111](0111-surfaces-and-floating-panes.md) (what floats under the pane's header).

## Context
The owner: "Ah maybe it's the toast saying it's checking for updates? It just visually looks like the no updates toast. We should also move the toast to the top instead of bottom." Toasts were Material snack bars at the window's bottom, centred on the whole window (not the main area), over the composer; the update card sat in the bottom right corner, over the same strip, and at 720 wide the two overlapped. "Checking for updates…" and "Nushi is up to date" were the same plain line.

## Decision
- **Toasts show at the top** on the Mac and Linux: 420 wide, centred over the chat's transcript (the composer's centre: what the details rail and a side pane leave of it; in a view that isn't a chat, the main area, right of the sidebar shown or hidden), 8 points under the header, under the bars when one shows. The bars stay centred over the main area. They slide down and fade in, and only fade as they go; under Reduce Motion they're there at once.
- **The app's own host**, not a snack bar (Flutter's only sit at the bottom): `ToastHost` above the navigator, with its queue (`Toasts`: one at a time, `replace` takes the showing one's place, a duration, an action holds it under VoiceOver) and an overlay of its own, so a toast is over every view, Settings and a dialog. The layout under the header marks the place (`ToastAnchor`, placed by `NoticesPlace`: `Beside` in a chat, `TopNotices` elsewhere) and the toast follows it; with no main area (the daemon starting) it's under where the header would be.
- **The update card sits at the transcript's top right**, under the header (so never over its buttons), 16 in from its edge: left of the details rail when the rail shows, so it never covers the rail's widgets; at the main area's top right when the rail is hidden or tucked, or in a view that isn't a chat. Where a toast or a bar centred there would meet it, in a narrow window, that one goes under the card.
- **A slow call's "running" toast has a spinner, its answer an icon**: Check for updates says "Checking for updates…" with a spinner, then "Nushi is up to date" with a green check (an info icon for no release or a development build), in the same place.
- **The phone keeps its toasts at the bottom**, near the thumb, the same host at the bottom edge.

## Consequences
- A toast covers the top of the transcript (or a view's first rows) for its seconds; that part of a chat is what's been read already.
- Tests' plain apps need the host (`themed` has it) to show a toast.

## Alternatives considered
- Toasts at the top right with the card, stacked: one column for every notice, but news would sit far from where the eye is, and a corner toast is easy to miss.
- Snack bars kept, turned upside down with a margin: Flutter's snack bar lays out from the bottom of the Scaffold; a top margin as tall as the window is fragile and still under a dialog.
