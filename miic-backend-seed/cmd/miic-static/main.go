package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"miic-portal/backend/internal/config"
	"miic-portal/backend/internal/demo"
	"miic-portal/backend/internal/generator"
	"miic-portal/backend/internal/httpapi"
	"miic-portal/backend/internal/repository"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return errors.New("用法: miic-static <preview|generate|serve> --config config.yaml")
	}
	command := args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "configuration file")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if command == "preview" {
		cfg.Site.DistRoot = cfg.Site.PreviewRoot
		g, err := generator.New(cfg, demo.NewSource(), logger)
		if err != nil {
			return err
		}
		result, err := g.GenerateSite(context.Background())
		if err == nil {
			logger.Info("演示站点生成完成", "output", result.Output)
		}
		return err
	}
	db, err := openDatabase(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	pageID, err := repository.ResolvePageID(context.Background(), db, cfg.Site.PageName)
	if err != nil {
		return err
	}
	g, err := generator.New(cfg, repository.New(db, pageID), logger)
	if err != nil {
		return err
	}
	switch command {
	case "generate":
		result, err := g.GenerateSite(context.Background())
		if err == nil {
			logger.Info("整站生成完成", "output", result.Output)
		}
		return err
	case "serve":
		return serve(cfg, g, logger)
	default:
		return fmt.Errorf("未知命令 %q，请使用 preview、generate 或 serve", command)
	}
}

func openDatabase(cfg config.Config) (*sql.DB, error) {
	dsn, err := cfg.DSN()
	if err != nil {
		return nil, err
	}
	mysqlCfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(cfg.Site.Timezone)
	if err != nil {
		return nil, err
	}
	mysqlCfg.ParseTime = true
	mysqlCfg.Loc = location
	db, err := sql.Open("mysql", mysqlCfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	life, _ := time.ParseDuration(cfg.Database.ConnMaxLifetime)
	db.SetConnMaxLifetime(life)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return db, nil
}

func serve(cfg config.Config, g *generator.Generator, logger *slog.Logger) error {
	token, err := cfg.Token()
	if err != nil {
		return err
	}
	requestTimeout, _ := time.ParseDuration(cfg.Server.RequestTimeout)
	idle, _ := time.ParseDuration(cfg.Server.BatchIdleTimeout)
	max, _ := time.ParseDuration(cfg.Server.BatchMaxDuration)
	service, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	handler := httpapi.New(service, g, token, requestTimeout, idle, max, logger)
	server := &http.Server{Addr: cfg.Server.Addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: requestTimeout + 10*time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("静态化服务已启动", "addr", cfg.Server.Addr)
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
