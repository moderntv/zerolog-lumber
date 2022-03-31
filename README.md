# Zerolog Logstash writer
This writer communicates with default logstash beats input.
As zerolog default writes the output in json the logstash input uses json codec.

# Logstash go client
This zerolog logstash writer uses go-lumber client. For more info visit [go-lumber repository](https://github.com/elastic/go-lumber).
As the client is non blocking, it uses buffer channel where each write processes the input and stores the result into buffer.
The asynchronous worker then processes the buffer when is full or when Flush is called, e.g:
```
lw.Flush(1*time.Second)
```

When the Flush is not called, the last bufferSize messages are definitelly lost. When the program wants to avoid using flushing, pass the context
of the main function into worker so it flushes the stream at the end of program.

# Tests
Tests are written with locally hosted logstash service. To run tests there have to be spawned
logstash service before running tests locally.
Simply run docker compose and tests
```
docker compose up -d
go test ./...
```
