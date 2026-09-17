package caamdb

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/builtin"
	"portal-static/internal/core/config"
	"portal-static/internal/platform"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type Options struct {
	DumpPath   string
	ConfigPath string
	Logger     *slog.Logger
}

type Summary struct {
	Pages             int    `json:"pages"`
	Columns           int    `json:"columns"`
	PublishedArticles int    `json:"published_articles"`
	DraftArticles     int    `json:"draft_articles"`
	GeneratedFiles    int    `json:"generated_files"`
	GeneratedDetails  int    `json:"generated_details"`
	GeneratedLists    int    `json:"generated_lists"`
	OutputScanned     string `json:"output_scanned"`
}

func Verify(ctx context.Context, options Options) (Summary, error) {
	if strings.TrimSpace(options.DumpPath) == "" {
		return Summary{}, errors.New("CAAM_SQL_DUMP is required")
	}
	if strings.TrimSpace(options.ConfigPath) == "" {
		options.ConfigPath = filepath.Join("configs", "caam.example.yaml")
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if err := dockerAvailable(ctx); err != nil {
		return Summary{}, err
	}

	tempRoot, err := os.MkdirTemp("", "portal-static-caam-verify-")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(tempRoot)
	extracted := filepath.Join(tempRoot, "caam_portal.sql")
	if err := extractPortalDatabase(options.DumpPath, extracted); err != nil {
		return Summary{}, err
	}

	password, err := randomToken()
	if err != nil {
		return Summary{}, err
	}
	container := "portal-static-caam-verify-" + password[:12]
	if _, err := docker(ctx, "run", "--detach", "--rm", "--name", container,
		"--label", "portal-static.verification=caam", "-e", "MYSQL_ROOT_PASSWORD="+password,
		"-p", "127.0.0.1::3306", "mysql:8.0",
		"--character-set-server=utf8mb4", "--collation-server=utf8mb4_0900_ai_ci"); err != nil {
		return Summary{}, fmt.Errorf("start isolated MySQL 8 container: %w", err)
	}
	defer func() { _, _ = docker(context.Background(), "rm", "--force", container) }()

	port, err := waitForMySQL(ctx, container, password)
	if err != nil {
		return Summary{}, err
	}
	if err := importSQL(ctx, container, password, extracted); err != nil {
		return Summary{}, err
	}

	dsnConfig := mysqlDriver.NewConfig()
	dsnConfig.User = "root"
	dsnConfig.Passwd = password
	dsnConfig.Net = "tcp"
	dsnConfig.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	dsnConfig.DBName = "caam_portal"
	dsnConfig.ParseTime = true
	dsnConfig.Loc = time.Local
	dsn := dsnConfig.FormatDSN()
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		return Summary{}, err
	}
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		return Summary{}, fmt.Errorf("connect imported CAAM database: %w", err)
	}

	summary, excludedIDs, err := inspectFixture(ctx, database)
	if err != nil {
		return Summary{}, err
	}
	manager, err := config.Open(options.ConfigPath)
	if err != nil {
		return Summary{}, fmt.Errorf("open CAAM config: %w", err)
	}
	snapshot := manager.Bootstrap()
	output := filepath.Join(tempRoot, "generated")
	snapshot.Paths.DistRoot = output
	snapshot.Paths.PreviewRoot = filepath.Join(tempRoot, "preview")
	const dsnEnv = "CAAM_VERIFY_DB_DSN"
	previous, existed := os.LookupEnv(dsnEnv)
	if err := os.Setenv(dsnEnv, dsn); err != nil {
		return Summary{}, err
	}
	defer func() {
		if existed {
			_ = os.Setenv(dsnEnv, previous)
		} else {
			_ = os.Unsetenv(dsnEnv)
		}
	}()
	snapshot.Database.DSNEnv = dsnEnv

	registry, err := builtin.Registry()
	if err != nil {
		return Summary{}, err
	}
	runtime, err := registry.Build(ctx, platform.Production, snapshot, options.Logger)
	if err != nil {
		return Summary{}, err
	}
	result, generationErr := runtime.Operations.GenerateSite(ctx)
	closeErr := runtime.Close()
	if generationErr != nil || closeErr != nil {
		return Summary{}, errors.Join(generationErr, closeErr)
	}
	var metrics struct {
		GeneratedFiles   int `json:"generated_files"`
		GeneratedDetails int `json:"generated_details"`
		GeneratedLists   int `json:"generated_lists"`
	}
	encoded, _ := json.Marshal(result)
	if err := json.Unmarshal(encoded, &metrics); err != nil {
		return Summary{}, fmt.Errorf("decode generation metrics: %w", err)
	}
	summary.GeneratedFiles = metrics.GeneratedFiles
	summary.GeneratedDetails = metrics.GeneratedDetails
	summary.GeneratedLists = metrics.GeneratedLists
	summary.OutputScanned = "temporary isolated directory"
	if err := validateOutput(output, summary.PublishedArticles, excludedIDs); err != nil {
		return Summary{}, err
	}
	return summary, nil
}

func dockerAvailable(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("Docker CLI is required for CAAM database verification")
	}
	if _, err := docker(ctx, "info", "--format", "{{.ServerVersion}}"); err != nil {
		return fmt.Errorf("Docker is not running; start Docker Desktop and retry: %w", err)
	}
	return nil
}

func docker(ctx context.Context, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "docker", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func waitForMySQL(ctx context.Context, container, password string) (int, error) {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if _, err := docker(ctx, "exec", "-e", "MYSQL_PWD="+password, container,
			"mysql", "-uroot", "--batch", "--skip-column-names", "-e", "SELECT 1"); err == nil {
			portOutput, err := docker(ctx, "port", container, "3306/tcp")
			if err != nil {
				return 0, err
			}
			lines := strings.Split(portOutput, "\n")
			_, rawPort, err := net.SplitHostPort(strings.TrimSpace(lines[0]))
			if err != nil {
				return 0, fmt.Errorf("parse Docker MySQL port %q: %w", portOutput, err)
			}
			port, err := strconv.Atoi(rawPort)
			if err != nil {
				return 0, err
			}
			return port, nil
		}
		time.Sleep(time.Second)
	}
	return 0, errors.New("isolated MySQL 8 did not become ready within two minutes")
}

func importSQL(ctx context.Context, container, password, path string) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	command := exec.CommandContext(ctx, "docker", "exec", "-i", "-e", "MYSQL_PWD="+password, container,
		"mysql", "-uroot", "--default-character-set=utf8mb4")
	command.Stdin = input
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("import caam_portal into isolated MySQL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func extractPortalDatabase(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open SQL dump: %w", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()

	reader := bufio.NewScanner(input)
	reader.Buffer(make([]byte, 64*1024), 64*1024*1024)
	writer := bufio.NewWriter(output)
	defer writer.Flush()
	var preamble []string
	seenDatabase := false
	inPortal := false
	writtenPortal := false
	for reader.Scan() {
		line := strings.TrimSuffix(reader.Text(), "\r")
		if strings.HasPrefix(line, "-- Current Database: ") {
			seenDatabase = true
			if inPortal {
				break
			}
			inPortal = strings.Contains(line, "`caam_portal`")
			if inPortal {
				for _, header := range preamble {
					_, _ = io.WriteString(writer, header+"\n")
				}
			}
		}
		if !seenDatabase {
			preamble = append(preamble, line)
		}
		if inPortal {
			writtenPortal = true
			_, _ = io.WriteString(writer, line+"\n")
		}
	}
	if err := reader.Err(); err != nil {
		return fmt.Errorf("read SQL dump: %w", err)
	}
	if !writtenPortal {
		return errors.New("SQL dump does not contain a caam_portal database section")
	}
	return nil
}

func inspectFixture(ctx context.Context, database *sql.DB) (Summary, []int64, error) {
	var summary Summary
	queries := []struct {
		target *int
		query  string
	}{
		{&summary.Pages, "SELECT COUNT(*) FROM page WHERE status=1 AND deleted_at IS NULL"},
		{&summary.Columns, "SELECT COUNT(*) FROM `column` WHERE deleted_at IS NULL"},
		{&summary.PublishedArticles, "SELECT COUNT(DISTINCT a.id) FROM article a INNER JOIN article_column_publish acp ON acp.article_id=a.id WHERE a.status=1 AND a.audit_status=2 AND a.deleted_at IS NULL AND a.type IN (1,2)"},
		{&summary.DraftArticles, "SELECT COUNT(*) FROM article WHERE deleted_at IS NOT NULL OR status<>1 OR audit_status<>2"},
	}
	for _, item := range queries {
		if err := database.QueryRowContext(ctx, item.query).Scan(item.target); err != nil {
			return Summary{}, nil, fmt.Errorf("inspect CAAM fixture: %w", err)
		}
	}
	if summary.Pages < 7 || summary.Columns == 0 || summary.PublishedArticles == 0 || summary.DraftArticles == 0 {
		return Summary{}, nil, fmt.Errorf("CAAM fixture is incomplete: %+v", summary)
	}
	rows, err := database.QueryContext(ctx, "SELECT id FROM article WHERE deleted_at IS NOT NULL OR status<>1 OR audit_status<>2 ORDER BY id")
	if err != nil {
		return Summary{}, nil, err
	}
	defer rows.Close()
	var excluded []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return Summary{}, nil, err
		}
		excluded = append(excluded, id)
	}
	return summary, excluded, rows.Err()
}

func validateOutput(root string, publishedArticles int, excludedIDs []int64) error {
	for _, name := range []string{"index.html", "about.html", "work.html", "stats.html", "members.html", "party.html"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.IsDir() || info.Size() == 0 {
			return fmt.Errorf("required CAAM page %s is missing or empty", name)
		}
	}
	articleFiles, err := filepath.Glob(filepath.Join(root, "article", "*", "*", "*.html"))
	if err != nil {
		return err
	}
	if len(articleFiles) != publishedArticles {
		return fmt.Errorf("generated article count %d does not match published fixture count %d", len(articleFiles), publishedArticles)
	}
	listFiles, err := filepath.Glob(filepath.Join(root, "list", "*", "*.html"))
	if err != nil || len(listFiles) == 0 {
		return errors.New("CAAM list output is empty")
	}
	excluded := make(map[string]struct{}, len(excludedIDs))
	for _, id := range excludedIDs {
		excluded[strconv.FormatInt(id, 10)+".html"] = struct{}{}
	}
	for _, path := range articleFiles {
		if _, exists := excluded[filepath.Base(path)]; exists {
			return fmt.Errorf("draft or unapproved article was generated: %s", filepath.Base(path))
		}
	}
	forbidden := []string{"10.1.100.138", "10.3.1.95", `D:\WebstormProjects`, "demo.miic.com.cn", "caamm/uploads"}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, value := range forbidden {
			if strings.Contains(text, value) {
				return fmt.Errorf("forbidden legacy value %q found in %s", value, path)
			}
		}
		return nil
	})
}

func randomToken() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
