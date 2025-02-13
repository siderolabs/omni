FROM ghcr.io/siderolabs/playwright-go:c3688e1

COPY go.* /tmp/test/
WORKDIR /tmp/test

RUN go mod download

COPY *.go /tmp/test/

RUN go test -c -o test main_test.go

CMD ["/tmp/test/test", "-test.v"]
