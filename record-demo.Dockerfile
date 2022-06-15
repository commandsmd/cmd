FROM alpine:3.16 as build

RUN apk add --update \
  coreutils \
  go \
  git \
  openssl \
  ca-certificates \
  && rm -rf /var/cache/apk/*

# TODO install tmux, asciinema, copy over cmd
ENV GOROOT /usr/lib/go
ENV GOPATH /gopath
ENV GOBIN /gopath/bin
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

WORKDIR /gopath/src/github.com/commandsmd/cmd
ADD go.mod go.sum /gopath/src/github.com/commandsmd/cmd/
RUN go mod download

ADD *.go /gopath/src/github.com/commandsmd/cmd/
ADD cmd/*.go /gopath/src/github.com/commandsmd/cmd/cmd/

RUN go build -o /usr/bin/cmd github.com/commandsmd/cmd/cmd

FROM alpine:3.16 as record
RUN apk add --update \
  asciinema \
  tmux \
  python3 \
  nodejs \
  bash \
  && ln -s /usr/bin/python3 /usr/bin/python \
  && rm -rf /var/cache/apk/*

WORKDIR /root
COPY --from=build /usr/bin/cmd /usr/bin/cmd
