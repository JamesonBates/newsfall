package app

import (
	"reflect"
	"testing"
)

func TestDecoderHandlesSplitArrowsTextAndControlKeys(t *testing.T) {
	var decoder Decoder
	if got := decoder.Feed([]byte{0x1b}); len(got) != 0 {
		t.Fatalf("partial escape emitted %#v", got)
	}
	got := decoder.Feed([]byte("[Ahi\r\x7f\t\x03"))
	want := []Key{
		{Kind: KeyUp},
		{Kind: KeyRune, Rune: 'h'},
		{Kind: KeyRune, Rune: 'i'},
		{Kind: KeyEnter},
		{Kind: KeyBackspace},
		{Kind: KeyTab},
		{Kind: KeyCtrlC},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %#v, want %#v", got, want)
	}
}

func TestDecoderPreservesSplitUTF8AndFlushesLoneEscape(t *testing.T) {
	var decoder Decoder
	bytes := []byte("λ")
	if got := decoder.Feed(bytes[:1]); len(got) != 0 {
		t.Fatalf("partial rune emitted %#v", got)
	}
	got := decoder.Feed(bytes[1:])
	if len(got) != 1 || got[0].Kind != KeyRune || got[0].Rune != 'λ' {
		t.Fatalf("UTF-8 = %#v", got)
	}
	if got := decoder.Feed([]byte{0x1b}); len(got) != 0 {
		t.Fatalf("partial escape = %#v", got)
	}
	if got := decoder.Flush(); !reflect.DeepEqual(got, []Key{{Kind: KeyEscape}}) {
		t.Fatalf("flush = %#v", got)
	}
}

func TestDecoderMapsCommonNavigationSequences(t *testing.T) {
	var decoder Decoder
	got := decoder.Feed([]byte("\x1b[B\x1b[C\x1b[D\x1b[H\x1b[F"))
	want := []Key{{Kind: KeyDown}, {Kind: KeyRight}, {Kind: KeyLeft}, {Kind: KeyHome}, {Kind: KeyEnd}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %#v", got)
	}
}
