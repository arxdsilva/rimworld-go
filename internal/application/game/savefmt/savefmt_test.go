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
