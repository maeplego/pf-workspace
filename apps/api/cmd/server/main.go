package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/auth"
	"github.com/portfolio/pf-workspace/api/internal/config"
	"github.com/portfolio/pf-workspace/api/internal/service"
	"github.com/portfolio/pf-workspace/api/internal/store/memory"
	"github.com/portfolio/pf-workspace/api/internal/web"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	store := memory.New()
	svc := service.New(store)
	svc.SetFileOpts(service.FileOpts{
		PublicURL:   cfg.PublicURL,
		MediaAPIURL: cfg.MediaAPIURL,
		UploadDir:   cfg.UploadDir,
	})
	mw := auth.New(cfg.DevAuth, cfg.OIDCIssuer, cfg.OIDCInternalBase, cfg.OIDCAudience)
	handler := web.New(svc, cfg.CORSOrigin, cfg.InternalToken, nil).Routes(mw)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("workspace api listening on %s (devAuth=%v)", cfg.HTTPAddr, cfg.DevAuth)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
}
