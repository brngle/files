build: build-css
  go build -o files-web-server cmd/files-web-server/main.go

build-css:
  tailwindcss -i ./index.css -o ./dist/index.css

watch-css:
  tailwindcss -i ./index.css -o ./dist/index.css --watch