FROM golang:alpine

RUN apk add --no-cache git

RUN addgroup app-user && \
    adduser -u 1001 -h /src -s /bin/false -G app-user -D -H app-user

ENV GOBIN /usr/bin

RUN go get "cloud.google.com/go/storage"
RUN go get "github.com/juju/loggo"
RUN go get "golang.org/x/crypto/openpgp"
RUN go get "github.com/alecthomas/kingpin"

COPY src/ /src

RUN go install /src/receiver/receiver.go && \
    go install /src/client/upload.go


USER 1001

ENTRYPOINT ["receiver"]