package main

import (
	"log/slog"
	"os"
)

func main() {
	b, err := newBuilder()
	if err != nil {
		slog.Error("initialization failed", "error", err)
		os.Exit(1)
	}

	if err := b.build(); err != nil {
		slog.Error("build failed", "error", err)
		os.Exit(1)
	}
}
