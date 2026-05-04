package main

import (
	"log/slog"
	"os"
)

func main() {
	slog.Info("Outerstellar Platform starting", "version", "dev")
	os.Exit(0)
}
