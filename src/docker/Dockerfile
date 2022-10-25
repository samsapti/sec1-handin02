FROM golang:1.19-alpine AS builder

COPY . /app
WORKDIR /app

RUN go mod download
RUN GOOS=linux CGO_ENABLED=0 go build -ldflags '-w' -o main .

FROM scratch

COPY --from=builder /app/main /main
COPY --chown=1000:1000 certs /certs

USER 1000

ENTRYPOINT [ "./main" ]