package savefmt

import (
	"bytes"
	"io"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := NewWriter(&buf)
	if _, err := zw.Write([]byte("save body")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil || string(got) != "save body" {
		t.Fatalf("round trip %q %v", got, err)
	}
	if _, err := NewReader(bytes.NewReader([]byte("not gzip"))); err == nil {
		t.Fatal("a file that is not gzip read as a save")
	}
}

func TestCompressIsNewWriter(t *testing.T) {
	t.Parallel()
	raw := bytes.Repeat([]byte("colony "), 1000)
	var want bytes.Buffer
	zw := NewWriter(&want)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := Compress(&got, raw); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("Compress differs from NewWriter")
	}
	if err := Compress(failWriter{}, raw); err == nil {
		t.Fatal("a failed write went unreported")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrShortWrite }
