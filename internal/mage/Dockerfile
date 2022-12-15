FROM golang:1.19-alpine3.17 AS builder

RUN apk update && apk add --no-cache \
  build-base
ADD . /src
WORKDIR /src
RUN go run main.go -compile /usr/bin/km

FROM alpine:3.17

ARG GITHUB_TOKEN
ARG GITHUB_USER_NAME="Kubo Mage"
ARG GITHUB_USER_EMAIL="noreply+kubo-mage@ipfs.tech"

ARG GPG_ID
ARG GPG_KEY
ARG GPG_PASSPHRASE=""

RUN apk update && apk add --no-cache \
  bash \
  git \
  gnupg \
  zsh
RUN git config --global gc.auto 0 && \
  git config --global protocol.version 2 && \
  git config --global core.sshCommand "" && \
  git config --global http.https://github.com/.extraheader "AUTHORIZATION: basic $(echo -n "pat:$GITHUB_TOKEN" | base64)" && \
  git config --global user.name "$GITHUB_USER_NAME" && \
  git config --global user.email "$GITHUB_USER_EMAIL" && \
  git config --global user.signingkey "$GPG_ID"
RUN echo "${GPG_KEY}" | gpg --batch --yes --import && \
  echo "use-agent" > ~/.gnupg/gpg.conf && \
  echo "no-tty" >> ~/.gnupg/gpg.conf

COPY --from=builder /usr/bin/km /usr/bin/km

ENV GITHUB_TOKEN $GITHUB_TOKEN
ENV GPG_KEY $GPG_KEY
ENV GPG_PASSPHRASE $GPG_PASSPHRASE

ENTRYPOINT [ "/usr/bin/km" ]
