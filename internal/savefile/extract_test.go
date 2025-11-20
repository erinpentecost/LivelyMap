package savefile

import (
	"bytes"
	"testing"
)

func TestExtractBetweenSimple(t *testing.T) {
	in := []byte("aaa PREFIX hello world SUFFIX bbb")
	got, err := extractBetween(bytes.NewReader(in), []byte("PREFIX "), []byte(" SUFFIX"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte("hello world")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractAcrossChunkBoundaries(t *testing.T) {
	// make a big input so reads will be chunked
	buf := bytes.Repeat([]byte("x"), 5000)
	// append prefix split across chunk boundary
	buf = append(buf, []byte("PR")...)           // end of chunk
	buf = append(buf, []byte("EFIX data SU")...) // start of next chunk (contains suffix split)
	buf = append(buf, []byte("FFIX tail")...)
	got, err := extractBetween(bytes.NewReader(buf), []byte("PREFIX "), []byte(" SUFFIX"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte("data")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrefixNotFound(t *testing.T) {
	_, err := extractBetween(bytes.NewReader([]byte("no prefix or suffix")), []byte("PREFIX"), []byte("SUFFIX"))
	if err == nil || err.Error() != "prefix not found" {
		t.Fatalf("expected 'prefix not found', got %v", err)
	}
}

func TestSuffixNotFound(t *testing.T) {
	_, err := extractBetween(bytes.NewReader([]byte("PREFIX but no suffix here")), []byte("PREFIX"), []byte("SUFFIX"))
	if err == nil || err.Error() != "suffix not found after prefix" {
		t.Fatalf("expected suffix-not-found error, got %v", err)
	}
}

func TestMultipleOccurrencesReturnsFirst(t *testing.T) {
	in := []byte("PREFIX one SUFFIX PREFIX two SUFFIX")
	got, err := extractBetween(bytes.NewReader(in), []byte("PREFIX "), []byte(" SUFFIX"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte("one")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
