# Newsfall Design

## Intent

Newsfall is an ambient terminal news stream: part TweetDeck, part live ticker, and visually closer to `btop` than to a conventional list-based RSS reader. It is designed to remain open in an idle terminal all day, but becomes a keyboard-driven reader when the user interacts with it.

## Experience

The default view is a responsive deck with one to three independently filtered columns. Wide terminals show three columns; narrower terminals collapse to two or one while preserving column navigation. A `stream` mode merges all articles into one chronological waterfall. A low-frequency ambient drift advances the cards after inactivity, while a live waveform and clock provide subtle continuous motion.

Each card includes colorful ANSI/Unicode cover art, title, source, age, category chips, excerpt, and a visible/clickable article URL. When a feed exposes an image, Newsfall renders it as a true-color half-block mosaic; otherwise it creates deterministic generative cover art from the article metadata. This works without Kitty, Sixel, iTerm2, or other terminal-specific image protocols.

## Controls

- `h`/`l` or arrows: change column
- `j`/`k` or arrows: move through cards
- `g`/`G`: first/last card
- `enter` or `o`: open article in the system browser
- `r`: refresh now
- `p`: pause/resume refresh and ambient drift
- `a`: toggle ambient drift
- `m`: toggle deck/stream
- `i`: toggle image mosaics
- `tab`: cycle theme
- `:`: enter a configuration command
- `?`: help
- `q` or `ctrl+c`: quit

In-app commands include `feed add/remove/list`, `column add/remove/list`, `topic add/remove`, `refresh`, `drift`, `theme`, `mode`, `images`, `ambient`, `reload`, and `help`. Changes are saved atomically to JSON.

## Architecture

Newsfall is a zero-dependency Go 1.23 application. A direct ANSI terminal runtime owns raw keyboard input, resize handling, the alternate screen, rendering, and the event loop. The remaining units are isolated packages for configuration, feed parsing/fetching, content cleanup and matching, cache persistence, commands, browser opening, and visual rendering.

RSS 0.9x/2.0, RSS 1.0/RDF, Atom, and JSON Feed are normalized into one `Article` model. Fetches are concurrent with bounded concurrency and per-request timeouts. A JSON cache in the XDG data directory provides useful offline startup. Individual feed failures are non-blocking and never erase the last useful screen.

Remote text is HTML-decoded, stripped of markup, normalized, and scrubbed of terminal control sequences before rendering. Browser opening uses argument-safe process APIs. Configuration and cache writes use temporary files plus atomic rename.

## Defaults

The first config contains three personalized starter columns: `AI + TECH`, `MACHINES`, and `GAMES + CULTURE`, with public starter feeds that can be removed immediately. An empty feed list displays setup instructions instead of failing; the desk itself always retains one to three columns.

## First-release limits

No accounts, cloud sync, social-network APIs, notifications, AI summaries, full-article extraction, or native terminal image protocols. The app remains local, inspectable, and useful without credentials or a server.
