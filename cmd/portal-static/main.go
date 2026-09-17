package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"portal-static/internal/adapters/builtin"
	"portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
	"portal-static/internal/platform"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	registry, err := builtin.Registry()
	if err != nil {
		return err
	}
	return runWithRegistry(args, registry)
}

func runWithRegistry(args []string, registry *platform.Registry) error {
	if len(args) < 2 {
		return errors.New("用法: portal-static <preview|generate|serve> --config configs/<site>.yaml")
	}
	command := args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "configuration file")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	var mode platform.Mode
	switch command {
	case "preview":
		mode = platform.Preview
	case "generate", "serve":
		mode = platform.Production
	default:
		return fmt.Errorf("未知命令 %q，请使用 preview、generate 或 serve", command)
	}
	manager, err := config.Open(*configPath)
	if err != nil {
		return err
	}
	snapshot := manager.Bootstrap()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if mode == platform.Preview {
		if snapshot.Paths.PreviewRoot == "" {
			return errors.New("paths.preview_root is required for preview")
		}
	}
	runtime, err := registry.Build(context.Background(), mode, snapshot, logger)
	if err != nil {
		return err
	}
	var runErr error
	switch command {
	case "preview":
		runErr = generateSite("preview", snapshot, runtime.Operations, logger)
	case "generate":
		runErr = generateSite("generate", snapshot, runtime.Operations, logger)
	case "serve":
		runErr = serve(snapshot, runtime.Operations, logger)
	}
	return errors.Join(runErr, runtime.Close())
}

func generateSite(mode string, snapshot config.Snapshot, operations httpapi.Operations, logger *slog.Logger) error {
	if operations.GenerateSite == nil {
		return fmt.Errorf("driver %q does not support site generation", snapshot.Driver)
	}
	result, err := operations.GenerateSite(context.Background())
	if err != nil {
		return err
	}
	logger.Info("整站生成完成", "mode", mode, "driver", snapshot.Driver, "site", snapshot.Site.ID, "result", result)
	return nil
}

func serve(snapshot config.Snapshot, operations httpapi.Operations, logger *slog.Logger) error {
	token := strings.TrimSpace(os.Getenv(snapshot.Server.TokenEnv))
	if snapshot.Server.TokenEnv == "" || token == "" {
		return fmt.Errorf("static token environment variable %q is empty", snapshot.Server.TokenEnv)
	}
	requestTimeout, _ := time.ParseDuration(snapshot.Server.RequestTimeout)
	idle, _ := time.ParseDuration(snapshot.Server.BatchIdleTimeout)
	maximum, _ := time.ParseDuration(snapshot.Server.BatchMaxDuration)
	service, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	handler := httpapi.NewWithOperations(service, operations, token, requestTimeout, idle, maximum, logger)
	server := &http.Server{
		Addr: snapshot.Server.Addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: requestTimeout + 10*time.Second, IdleTimeout: 60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("静态化服务已启动", "addr", snapshot.Server.Addr, "site", snapshot.Site.ID)
		errCh <- server.ListenAndServe()
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	cancelJobs()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}
