package zerolog_logstash

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	lumber "github.com/elastic/go-lumber/client/v2"
	"github.com/rs/zerolog"
)

func createMultiLevelWriters(t *testing.T, bufferSize int, cancel context.CancelFunc, context context.Context) (w *Writer, z zerolog.Logger) {
	client, err := lumber.Dial("localhost:5044")
	if err != nil {
		panic("could not dial logstash service")
	}
	t.Cleanup(func() {
		if cancel != nil {
			cancel()
		}
		time.Sleep(1 * time.Second)
		client.Close()
	})

	w = NewWriter(client, WithBufferSize(bufferSize), WithContext(context))
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
	w, z := createMultiLevelWriters(t, 30, nil, context.Background())
	str := "string"
	z.Trace().
		Str("str", str).
		Msg("abcde")
	w.Flush(1 * time.Second)
}

func TestParallelLoggingWithContext(t *testing.T) {
	context, cancel := context.WithCancel(context.Background())
	_, z := createMultiLevelWriters(t, 7, cancel, context)
	for i := 0; i < 5; i++ {
		go func(j int) {
			z.Warn().
				Int("integer", j).
				Msg("newVar")
		}(i)
	}
	time.Sleep(700 * time.Millisecond)
}

func TestFlush(t *testing.T) {
	w, z := createMultiLevelWriters(t, 5, nil, context.Background())
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
