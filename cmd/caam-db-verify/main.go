package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"portal-static/internal/verification/caamdb"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	logOutput := io.Writer(io.Discard)
	if os.Getenv("CAAM_VERIFY_VERBOSE") == "1" {
		logOutput = os.Stdout
	}
	summary, err := caamdb.Verify(ctx, caamdb.Options{
		DumpPath: os.Getenv("CAAM_SQL_DUMP"), ConfigPath: os.Getenv("CAAM_VERIFY_CONFIG"),
		Logger: slog.New(slog.NewJSONHandler(logOutput, nil)),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(output))
}
