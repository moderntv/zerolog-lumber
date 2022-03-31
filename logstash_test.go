package zerolog_logstash

import (
	"io"
	"os"
	"testing"
	"time"

	lumber "github.com/elastic/go-lumber/client/v2"
	"github.com/rs/zerolog"
)

func createMultiLevelWriters(t *testing.T, bufferSize int) (w *Writer, z zerolog.Logger) {
	client, err := lumber.Dial("localhost:5044")
	if err != nil {
		panic("could not dial logstash service")
	}
	t.Cleanup(func() {
		client.Close()
	})

	w = NewWriter(client, WithBufferSize(bufferSize))
	writers := []io.Writer{
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
		w,
	}
	z = zerolog.New(zerolog.MultiLevelWriter(writers...)).With().Timestamp().Logger()
	return
}

func TestSimpleString(t *testing.T) {
	w, z := createMultiLevelWriters(t, 30)
	str := "string"
	z.Trace().
		Str("str", str).
		Msg("abcde")
	w.Flush(1 * time.Second)
}

func TestParallelLogging(t *testing.T) {
	w, z := createMultiLevelWriters(t, 7)
	for i := 0; i < 5; i++ {
		go func(j int) {
			z.Warn().
				Int("integer", j).
				Msg("newVar")
		}(i)
	}
	time.Sleep(700 * time.Millisecond)
	flushed := w.Flush(1 * time.Second)
	for !flushed {
		flushed = w.Flush(1 * time.Second)
	}
}

func TestFlush(t *testing.T) {
	w, z := createMultiLevelWriters(t, 5)
	for i := 0; i < 5; i++ {
		go func(j int) {
			z.Warn().
				Msg("flushing")
		}(i)
	}
	time.Sleep(300 * time.Millisecond)
	flushed := w.Flush(1 * time.Second)
	for !flushed {
		flushed = w.Flush(1 * time.Second)
	}
}
