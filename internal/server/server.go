package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/laraxpy/go-backend-starter/internal/config"
)

func New(r *gin.Engine, cfg *config.Config) *http.Server{
	return &http.Server{
		Addr: ":" + cfg.Port,
		Handler: r,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
}

func Run(srv *http.Server){
	quit := make(chan os.Signal,1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func(){
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed{
			slog.Error("Error al iniciar el servidor ","error", err)
			os.Exit(1)
		}
	}()
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("error al apagar el servidor: ","error", err)
	}
}