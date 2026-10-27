package ajean

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("Descripteur non valide") }

// #177 : une console absente ne doit pas faire échouer l'étape, et le journal
// doit quand même tout recevoir.
func TestLenientWriterIgnoresBrokenConsole(t *testing.T) {
	var log bytes.Buffer
	w := io.MultiWriter(lenientWriter{failWriter{}}, &log)
	n, err := w.Write([]byte("ligne\n"))
	if err != nil || n != 6 || log.String() != "ligne\n" {
		t.Fatalf("n=%d err=%v log=%q", n, err, log.String())
	}
}
