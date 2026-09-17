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
	"strings"
	"syscall"
	"time"

	"portal-static/internal/adapters/caam"
	"portal-static/internal/adapters/miic"
	miicdemo "portal-static/internal/adapters/miic/demo"
	miicrepository "portal-static/internal/adapters/miic/repository"
	"portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"

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
		return errors.New("用法: portal-static <preview|generate|serve> --config configs/miic.yaml")
	}
	command := args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := flags.String("config", "config.yaml", "configuration file")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	manager, err := config.Open(*configPath)
	if err != nil {
		return err
	}
	snapshot := manager.Bootstrap()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if command == "preview" {
		if snapshot.Paths.PreviewRoot == "" {
			return errors.New("paths.preview_root is required for preview")
		}
		operations, err := previewOperations(snapshot, logger)
		if err != nil {
			return err
		}
		return generateSite("preview", snapshot, operations, logger)
	}

	database, err := openDatabase(snapshot)
	if err != nil {
		return err
	}
	defer database.Close()
	var operations httpapi.Operations
	switch command {
	case "generate":
		operations, err = productionOperations(snapshot, database, logger)
		if err != nil {
			return err
		}
		return generateSite("generate", snapshot, operations, logger)
	case "serve":
		operations, err = productionOperations(snapshot, database, logger)
		if err != nil {
			return err
		}
		return serve(snapshot, operations, logger)
	default:
		return fmt.Errorf("未知命令 %q，请使用 preview、generate 或 serve", command)
	}
}

func previewOperations(snapshot config.Snapshot, logger *slog.Logger) (httpapi.Operations, error) {
	switch snapshot.Driver {
	case "miic":
		cfg, err := miic.FromSnapshot(snapshot)
		if err != nil {
			return httpapi.Operations{}, err
		}
		cfg.Site.DistRoot = cfg.Site.PreviewRoot
		service, err := miic.New(cfg, miicdemo.NewSource(), logger)
		if err != nil {
			return httpapi.Operations{}, err
		}
		return httpapi.OperationsForGenerator(service, miic.NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们"), nil
	case "caam":
		_, service, err := caam.NewPreview(snapshot, logger)
		if err != nil {
			return httpapi.Operations{}, err
		}
		return caam.Operations(service), nil
	default:
		return httpapi.Operations{}, fmt.Errorf("driver %q is not implemented; available: miic, caam", snapshot.Driver)
	}
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

func productionOperations(snapshot config.Snapshot, database *sql.DB, logger *slog.Logger) (httpapi.Operations, error) {
	switch snapshot.Driver {
	case "miic":
		cfg, err := miic.FromSnapshot(snapshot)
		if err != nil {
			return httpapi.Operations{}, err
		}
		pageID, err := miicrepository.ResolvePageID(context.Background(), database, cfg.Site.PageName)
		if err != nil {
			return httpapi.Operations{}, err
		}
		service, err := miic.New(cfg, miicrepository.New(database, pageID), logger)
		if err != nil {
			return httpapi.Operations{}, err
		}
		return httpapi.OperationsForGenerator(service, miic.NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们"), nil
	case "caam":
		_, service, err := caam.NewProduction(snapshot, database, logger)
		if err != nil {
			return httpapi.Operations{}, err
		}
		return caam.Operations(service), nil
	default:
		return httpapi.Operations{}, fmt.Errorf("driver %q is not implemented; available: miic, caam", snapshot.Driver)
	}
}

func openDatabase(snapshot config.Snapshot) (*sql.DB, error) {
	dsn := strings.TrimSpace(os.Getenv(snapshot.Database.DSNEnv))
	if snapshot.Database.DSNEnv == "" || dsn == "" {
		return nil, fmt.Errorf("database DSN environment variable %q is empty", snapshot.Database.DSNEnv)
	}
	mysqlConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(snapshot.Site.Timezone)
	if err != nil {
		return nil, err
	}
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = location
	database, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(snapshot.Database.MaxOpenConns)
	database.SetMaxIdleConns(snapshot.Database.MaxIdleConns)
	lifetime, _ := time.ParseDuration(snapshot.Database.ConnMaxLifetime)
	database.SetConnMaxLifetime(lifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return database, nil
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
