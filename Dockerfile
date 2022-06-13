FROM golang:1.18.3 AS build-env
WORKDIR /go/src/github.com/fairwindsops/gemini/

ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -a -o gemini

FROM alpine:3.15
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates

RUN addgroup -S gemini && adduser -u 1200 -S gemini -G gemini
USER 1200
COPY --from=build-env /go/src/github.com/fairwindsops/gemini/gemini .

WORKDIR /opt/app

CMD ["gemini"]
