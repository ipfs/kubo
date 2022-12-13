FROM alpine:3.16

RUN apk update && apk add --no-cache \
  bash \
  git

ARG GITHUB_TOKEN
ARG GITHUB_USER_NAME="Kubo Mage"
ARG GITHUB_USER_EMAIL="noreply+kubo-mage@ipfs.tech"

RUN git config --global gc.auto 0 \
  git config --global protocol.version 2 \
  git config --global core.sshCommand "" \
  git config --global http.https://github.com/.extraheader "AUTHORIZATION: basic $(echo "pat:$GITHUB_TOKEN" | base64)" \
  git config --global user.name "$GITHUB_USER_NAME" \
  git config --global user.email "$GITHUB_USER_EMAIL"
