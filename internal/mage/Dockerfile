FROM golang:1.19-alpine3.17 AS builder

RUN apk update && apk add --no-cache \
  build-base
ADD . /src
WORKDIR /src
RUN go run main.go -compile /usr/bin/km

FROM alpine:3.17

RUN apk update && apk add --no-cache \
  bash \
  zsh \
  npm

COPY --from=builder /usr/bin/km /usr/bin/km

ENTRYPOINT [ "/usr/bin/km" ]
