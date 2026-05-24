package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirotin/amnezia-config-watcher/internal/watch"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "check":
		runCheck(os.Args[2:])
	case "decode":
		runDecode(os.Args[2:])
	case "notify-test":
		runNotifyTest(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `amnezia-config-watch %s

Commands:
  serve        run the local web app and scheduler
  check        run one account-info check
  decode       decode a vpn:// key and print redacted JSON
  notify-test  send a Telegram test notification
  status       print redacted config/state status
  version      print version

`, version)
}

type commonOptions struct {
	paths   *watch.Paths
	workdir *string
	envFile *string
}

func commonFlags(name string, args []string) (*flag.FlagSet, commonOptions) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	paths := watch.DefaultPaths()
	workdir := fs.String("workdir", "", "directory for config.json and state.json")
	envFile := fs.String("env-file", ".env", "dotenv file to load before running")
	fs.StringVar(&paths.ConfigPath, "config", paths.ConfigPath, "config JSON path")
	fs.StringVar(&paths.StatePath, "state", paths.StatePath, "state JSON path")
	return fs, commonOptions{paths: &paths, workdir: workdir, envFile: envFile}
}

func applyCommonOptions(opts commonOptions) *watch.Paths {
	if err := watch.LoadEnvFile(*opts.envFile); err != nil {
		fatal(err)
	}
	opts.paths.ApplyWorkdir(*opts.workdir)
	return opts.paths
}

func runServe(args []string) {
	fs, opts := commonFlags("serve", args)
	var listenOverride, fixture string
	fs.StringVar(&listenOverride, "listen", "", "override listen address")
	fs.StringVar(&fixture, "fixture-account-info", "", "read account_info JSON from this file instead of Amnezia gateway")
	fs.Parse(args)
	paths := applyCommonOptions(opts)

	cfg, err := watch.LoadConfig(paths.ConfigPath)
	if err != nil {
		fatal(err)
	}
	if listenOverride != "" {
		cfg.ListenAddr = listenOverride
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = watch.DefaultListenAddr
	}
	setupToken := ""
	if cfg.Web.PasswordHash == "" {
		setupToken, err = watch.GenerateSetupToken()
		if err != nil {
			fatal(err)
		}
		log.Printf("first-run setup token: %s", setupToken)
		log.Printf("open http://%s/?setup_token=%s", cfg.ListenAddr, setupToken)
	}

	app := watch.NewApp(paths, cfg, fixture, setupToken)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Serve(ctx); err != nil {
		fatal(err)
	}
}

func runCheck(args []string) {
	fs, opts := commonFlags("check", args)
	var fixture string
	fs.StringVar(&fixture, "fixture-account-info", "", "read account_info JSON from this file instead of Amnezia gateway")
	fs.Parse(args)
	paths := applyCommonOptions(opts)

	cfg, err := watch.LoadConfig(paths.ConfigPath)
	if err != nil {
		fatal(err)
	}
	app := watch.NewApp(paths, cfg, fixture, "")
	result, err := app.Check(context.Background(), true)
	if err != nil {
		fatal(err)
	}
	fmt.Println(watch.MustJSON(result))
}

func runDecode(args []string) {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	var key string
	var showSecrets bool
	fs.StringVar(&key, "key", "", "vpn:// key; stdin is used when empty")
	fs.BoolVar(&showSecrets, "show-secrets", false, "print decoded secrets")
	fs.Parse(args)
	if key == "" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fatal(err)
		}
		key = strings.TrimSpace(string(b))
	}
	decoded, err := watch.DecodeVPNKey(key)
	if err != nil {
		fatal(err)
	}
	if !showSecrets {
		decoded = watch.RedactValue(decoded).(map[string]any)
	}
	fmt.Println(watch.MustJSON(decoded))
}

func runNotifyTest(args []string) {
	fs, opts := commonFlags("notify-test", args)
	fs.Parse(args)
	paths := applyCommonOptions(opts)
	cfg, err := watch.LoadConfig(paths.ConfigPath)
	if err != nil {
		fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := watch.SendTelegram(ctx, cfg.Telegram, "Amnezia Config Watcher test notification"); err != nil {
		fatal(err)
	}
	fmt.Println("ok")
}

func runStatus(args []string) {
	fs, opts := commonFlags("status", args)
	fs.Parse(args)
	paths := applyCommonOptions(opts)
	cfg, err := watch.LoadConfig(paths.ConfigPath)
	if err != nil {
		fatal(err)
	}
	state, err := watch.LoadState(paths.StatePath)
	if err != nil {
		fatal(err)
	}
	fmt.Println(watch.MustJSON(map[string]any{
		"config": watch.RedactValue(cfg),
		"state":  state,
	}))
}

func fatal(err error) {
	log.Printf("error: %v", err)
	os.Exit(1)
}
