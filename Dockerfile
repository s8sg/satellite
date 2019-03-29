FROM golang:1.10 as build

WORKDIR /go/src/github.com/s8sg/satellite

COPY .git             .git
COPY vendor             vendor
COPY pkg                pkg
COPY main.go            .

ARG GIT_COMMIT
ARG VERSION

RUN test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*"))" || { echo "Run \"gofmt -s -w\" on your Golang code"; exit 1; }

RUN CGO_ENABLED=0 go build -ldflags "-s -w \
      -X main.GitCommit=${GIT_COMMIT} -X main.Version=${VERSION}" \
      -a -installsuffix cgo -o /usr/bin/satellite

FROM alpine:3.9
RUN apk add --force-refresh ca-certificates

COPY --from=build /usr/bin/satellite /root/

EXPOSE 80

WORKDIR /root/

CMD ["/usr/bin/satellite"]
