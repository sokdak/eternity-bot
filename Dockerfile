FROM golang:1.23-alpine as builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY main.go main.go
COPY pkg/ pkg/

RUN apk add --no-cache gcc musl-dev
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -ldflags='-s -w -extldflags "-static"' -a -o eternity-bot main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/eternity-bot .
USER 65532:65532

ENTRYPOINT ["/eternity-bot"]
