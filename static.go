package main

import (
	"embed"
)

//go:embed static/*
var staticFS embed.FS
