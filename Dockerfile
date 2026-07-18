# Build
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git curl make

RUN go install github.com/a-h/templ/cmd/templ@latest
RUN go install github.com/air-verse/air@latest

ENV GO111MODULE=on
ENV CGO_ENABLED=0

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN templ generate && go build -ldflags="-s -w" -o bin/server ./cmd/server

EXPOSE 8080 4000


# Deploy
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /opt/nutmeg
COPY --from=build /go/src/app/bin/server /opt/nutmeg/server
COPY --from=build /go/src/app/static /opt/nutmeg/static

EXPOSE 8080

CMD ["./server"]
