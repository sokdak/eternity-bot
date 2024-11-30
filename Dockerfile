FROM golang:1.21 as builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY main.go main.go
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -o eternity-bot main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/eternity-bot .
USER 65532:65532

ENTRYPOINT ["/eternity-bot"]
