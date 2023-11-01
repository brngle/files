package files

import "embed"

//go:embed dist/*
var dist embed.FS

//go:embed static/*
var templates embed.FS
