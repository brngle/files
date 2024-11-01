FROM golang:1.23-alpine

RUN mkdir -p /usr/src/
WORKDIR /usr/src/

RUN apk add --no-cache --update curl

RUN curl -fsSL https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.14/tailwindcss-linux-x64 -o /bin/tailwindcss && chmod +x /bin/tailwindcss
COPY go.mod go.sum /usr/src/files/

WORKDIR /usr/src/files
RUN go mod download

COPY . /usr/src/files/

RUN tailwindcss -i ./index.css -o ./dist/index.css
RUN go build -v -o /bin/files-web-server cmd/files-web-server/main.go

ENTRYPOINT ["/bin/files-web-server"]
