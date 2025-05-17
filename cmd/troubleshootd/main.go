package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	eapi "go.sia.tech/explored/api"
	"go.sia.tech/host-troubleshoot/api"
	"go.sia.tech/host-troubleshoot/build"
	"go.sia.tech/host-troubleshoot/troubleshoot"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// humanEncoder returns a zapcore.Encoder that encodes logs as human-readable
// text.
func humanEncoder(showColors bool) zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder
	cfg.EncodeDuration = zapcore.StringDurationEncoder

	if showColors {
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	cfg.StacktraceKey = ""
	cfg.CallerKey = ""
	return zapcore.NewConsoleEncoder(cfg)
}

func main() {
	var (
		httpAddr string

		exploredAPIAddress  string
		exploredAPIPassword string

		logLevel zap.AtomicLevel
	)

	flag.StringVar(&httpAddr, "http.addr", ":8080", "HTTP address to listen on")
	flag.StringVar(&exploredAPIAddress, "api.address", "https://api.siascan.com", "Explored API address")
	flag.StringVar(&exploredAPIPassword, "api.password", "", "Explored API password")
	flag.TextVar(&logLevel, "log.level", zap.NewAtomicLevelAt(zapcore.InfoLevel), "Log level (debug, info, warn, error)")
	flag.Parse()

	core := zapcore.NewCore(humanEncoder(true), zapcore.Lock(os.Stdout), logLevel)
	log := zap.New(core, zap.AddCaller())
	defer log.Sync()

	zap.RedirectStdLog(log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	exploredClient := eapi.NewClient(exploredAPIAddress, exploredAPIPassword)

	t, err := troubleshoot.NewManager(exploredClient, log.Named("troubleshoot"))
	if err != nil {
		log.Fatal("failed to create troubleshoot manager", zap.Error(err))
	}
	defer t.Close()

	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}
	defer l.Close()

	srv := &http.Server{
		ReadTimeout: 10 * time.Second,
		Handler:     api.NewHandler(t),
	}
	defer srv.Close()
	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to serve", zap.Error(err))
		}
	}()

	log.Info("troubleshoot server started", zap.String("http", l.Addr().String()), zap.String("version", build.Version()))
	<-ctx.Done()
	log.Info("shutting down server")
}
