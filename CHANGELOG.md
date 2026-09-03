# Changelog

## 0.1.1 — 2026-09-03

- Keep terminal input alive after idle `VMIN`/`VTIME` reads are surfaced by Go as `io.EOF`.
- Emit interactive frames with CRLF row separators so raw-mode rendering begins each row at column one in Warp and other terminal emulators.
- Format the live header using the actual local weekday and month.
- Add a delayed-input pseudo-terminal regression test and a raw-frame line-ending regression test.

## 0.1.0 — 2026-09-03

- Initial Newsfall release.
