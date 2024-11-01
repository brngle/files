FROM golang:1.23-alpine

RUN mkdir -p /usr/src/
WORKDIR /usr/src/

COPY go.mod go.sum /usr/src/files/

WORKDIR /usr/src/files
RUN go mod download

COPY . /usr/src/files/

RUN go build -v -o /bin/files-web-server cmd/files-web-server/main.go

ENTRYPOINT ["/bin/files-web-server"]
