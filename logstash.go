package zerolog_logstash

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	lumber "github.com/elastic/go-lumber/client/v2"
)

// Option function
type Option func(*options)

type options struct {
	fields     map[string]string
	bufferSize int
	ctx        context.Context
}

func newDefaultOptions() options {
	return options{
		fields: map[string]string{
			"type":    "log",
			"address": "localhost:5044",
		},
		bufferSize: 30,
		ctx:        context.Background(),
	}
}

// IP Address with port of running instance.
func WithAddress(value string) Option {
	return func(o *options) {
		o.fields["address"] = value
	}
}

// Buffer size that specifies how big batches are processed
// written to logstash service at once.
// Minimum is 1
func WithBufferSize(value int) Option {
	return func(o *options) {
		o.bufferSize = value
	}
}

// WithContext should be used when context will expire/ be canceled
// so the rest of the buffer is sent to logstash
// Alternativelly use Flush(time.Duration) function
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.ctx = ctx
	}

}

// Batch that contains items to write into logstash.
// When error occurs the error channel contains message
// which specifies the item that was somehow corrupted
type batch struct {
	items   chan interface{}
	started chan struct{}
	done    chan struct{}
}

// Logstash writer using lumber Client which is default input filter in logstash service
type Writer struct {
	client     *lumber.Client
	buffer     chan batch
	bufferSize int
	start      sync.Once
	fields     map[string]string
}

func NewWriter(client *lumber.Client, opts ...Option) *Writer {
	options := newDefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	lw := &Writer{
		client:     client,
		buffer:     make(chan batch, 1),
		bufferSize: options.bufferSize,
		fields:     options.fields,
	}
	lw.buffer <- batch{
		items:   make(chan interface{}, options.bufferSize),
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
	lw.start.Do(func() {
		go lw.worker(options.ctx)
	})
	return lw
}

func (lw *Writer) sendItems(toSend []interface{}) (skip bool) {
	err := lw.client.Send(toSend)
	if err != nil {
		log.Println("Unable to send: ", err)
		return true
	}

	return false
}

func (lw *Writer) worker(ctx context.Context) {
	var closedItems = false
	for b := range lw.buffer {
		var toSend []interface{}
		close(b.started)
		// Release buffer so others can insert to items
		lw.buffer <- b
		for {
			if closedItems {
				closedItems = false
				break
			}

			select {
			case item := <-b.items:
				if item == nil {
					closedItems = true
					continue
				}

				// When buffer is full, flush it immediatelly
				if len(toSend) == lw.bufferSize {
					lw.sendItems(toSend)
					toSend = nil
				}

				toSend = append(toSend, item)

			// End of application
			case <-ctx.Done():
				// Drain buffer
				<-lw.buffer
				closedItems = true
				close(b.items)
			}
		}

		lw.sendItems(toSend)
		close(b.done)
	}
}

func (lw *Writer) process(p []byte) (sending interface{}, err error) {
	// Add optional fields of LogstashWriter
	var data map[string]interface{}
	err = json.Unmarshal(p, &data)
	if err != nil {
		return
	}

	for k, v := range lw.fields {
		data[k] = v
	}
	// Marshal and unmarshal them because of lumber Send([]interface{}) input argument
	newData, err := json.Marshal(data)
	if err != nil {
		return
	}

	newData = append(newData, '\n')
	err = json.Unmarshal(newData, &sending)
	return
}

func (lw *Writer) Write(p []byte) (n int, err error) {
	sending, err := lw.process(p)
	if err != nil {
		return
	}

	b := <-lw.buffer
	select {
	// Send items to items channel
	case b.items <- sending:
	default:
		log.Println("Buffer is full")
	}
	lw.buffer <- b
	return len(p), err
}

// Flush rest of the items
func (lw *Writer) Flush(timeout time.Duration) bool {
	toolate := time.After(timeout)
	var b batch
	for {
		select {
		case b = <-lw.buffer:
			select {
			case <-b.started:
				close(b.items)
				lw.buffer <- batch{
					items:   make(chan interface{}, lw.bufferSize),
					started: make(chan struct{}),
					done:    make(chan struct{}),
				}
				select {
				case <-b.done:
					return true
				case <-toolate:
					log.Println("too late")
					return false
				}
			}
		case <-toolate:
			log.Println("too late")
			return false
		}
	}
}
