# Changelog

## 0.1.2 — 2026-09-03

- Accept ordinary website URLs in `feed add`, discover advertised RSS, Atom, or JSON Feed endpoints, and fall back to common feed paths.
- Persist successfully discovered feed endpoints so later refreshes fetch them directly.
- Queue a follow-up synchronization when a source is added during an in-progress refresh instead of silently discarding that request.
- Allow unquoted multiword source names and infer a final existing column ID.
- Surface command failures in a visible overlay instead of only the footer.
- Add `e` and `feed errors` for full source diagnostics, with an explicit footer prompt whenever a source fails.
- Replace opaque parser failures for ordinary web pages with actionable feed-discovery guidance.

## 0.1.1 — 2026-09-03

- Keep terminal input alive after idle `VMIN`/`VTIME` reads are surfaced by Go as `io.EOF`.
- Emit interactive frames with CRLF row separators so raw-mode rendering begins each row at column one in Warp and other terminal emulators.
- Format the live header using the actual local weekday and month.
- Add a delayed-input pseudo-terminal regression test and a raw-frame line-ending regression test.

## 0.1.0 — 2026-09-03

- Initial Newsfall release.
