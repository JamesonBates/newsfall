package app

import "unicode/utf8"

// KeyKind represents one normalized terminal input event.
type KeyKind uint8

const (
	KeyUnknown KeyKind = iota
	KeyRune
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyEnter
	KeyBackspace
	KeyDelete
	KeyEscape
	KeyTab
	KeyCtrlC
)

// Key carries either a named key or a printable rune.
type Key struct {
	Kind KeyKind
	Rune rune
}

// Decoder turns arbitrarily chunked terminal bytes into key events.
type Decoder struct {
	pending []byte
}

func (d *Decoder) Feed(data []byte) []Key {
	d.pending = append(d.pending, data...)
	var keys []Key
	for len(d.pending) > 0 {
		b := d.pending[0]
		if b == 0x1b {
			consumed, key, complete := decodeEscape(d.pending)
			if !complete {
				break
			}
			d.pending = d.pending[consumed:]
			if key.Kind != KeyUnknown {
				keys = append(keys, key)
			}
			continue
		}
		switch b {
		case 0x03:
			keys = append(keys, Key{Kind: KeyCtrlC})
			d.pending = d.pending[1:]
			continue
		case '\r', '\n':
			keys = append(keys, Key{Kind: KeyEnter})
			d.pending = d.pending[1:]
			continue
		case '\t':
			keys = append(keys, Key{Kind: KeyTab})
			d.pending = d.pending[1:]
			continue
		case 0x08, 0x7f:
			keys = append(keys, Key{Kind: KeyBackspace})
			d.pending = d.pending[1:]
			continue
		}
		if b < utf8.RuneSelf {
			d.pending = d.pending[1:]
			if b >= 0x20 {
				keys = append(keys, Key{Kind: KeyRune, Rune: rune(b)})
			}
			continue
		}
		if !utf8.FullRune(d.pending) {
			break
		}
		r, size := utf8.DecodeRune(d.pending)
		d.pending = d.pending[size:]
		if r != utf8.RuneError {
			keys = append(keys, Key{Kind: KeyRune, Rune: r})
		}
	}
	return keys
}

// Flush resolves an otherwise ambiguous lone escape key after an idle read.
func (d *Decoder) Flush() []Key {
	if len(d.pending) == 0 {
		return nil
	}
	pending := append([]byte(nil), d.pending...)
	d.pending = nil
	var keys []Key
	if pending[0] == 0x1b {
		keys = append(keys, Key{Kind: KeyEscape})
		pending = pending[1:]
	}
	if len(pending) > 0 {
		keys = append(keys, d.Feed(pending)...)
		if len(d.pending) > 0 {
			r, size := utf8.DecodeRune(d.pending)
			if size > 0 && r != utf8.RuneError {
				keys = append(keys, Key{Kind: KeyRune, Rune: r})
			}
			d.pending = nil
		}
	}
	return keys
}

func decodeEscape(data []byte) (consumed int, key Key, complete bool) {
	if len(data) < 2 {
		return 0, Key{}, false
	}
	if data[1] != '[' && data[1] != 'O' {
		return 1, Key{Kind: KeyEscape}, true
	}
	if data[1] == 'O' {
		if len(data) < 3 {
			return 0, Key{}, false
		}
		return 3, keyForFinal(data[2]), true
	}
	for i := 2; i < len(data); i++ {
		if data[i] < 0x40 || data[i] > 0x7e {
			continue
		}
		final := data[i]
		if final == '~' {
			params := string(data[2:i])
			switch params {
			case "3":
				return i + 1, Key{Kind: KeyDelete}, true
			case "1", "7":
				return i + 1, Key{Kind: KeyHome}, true
			case "4", "8":
				return i + 1, Key{Kind: KeyEnd}, true
			}
		}
		return i + 1, keyForFinal(final), true
	}
	return 0, Key{}, false
}

func keyForFinal(final byte) Key {
	switch final {
	case 'A':
		return Key{Kind: KeyUp}
	case 'B':
		return Key{Kind: KeyDown}
	case 'C':
		return Key{Kind: KeyRight}
	case 'D':
		return Key{Kind: KeyLeft}
	case 'H':
		return Key{Kind: KeyHome}
	case 'F':
		return Key{Kind: KeyEnd}
	default:
		return Key{Kind: KeyUnknown}
	}
}
