package readiness

import (
	"context"
	"github.com/ONSdigital/ssdc-rm-pubsub-adapter/logger"
	"os"
)

type Readiness struct {
	FilePath string
	IsReady  bool
}

func New(ctx context.Context, filePath string) *Readiness {
	readiness := &Readiness{FilePath: filePath, IsReady: false}
	go readiness.removeReadyWhenDone(ctx)
	return readiness
}

func (r *Readiness) Ready() error {
	if _, err := os.Stat(r.FilePath); err == nil {
		logger.Logger.Warnw("Readiness file already existed", "readinessFilePath", r.FilePath)
	}
	if _, err := os.Create(r.FilePath); err != nil {
		return err
	}
	r.IsReady = true
	return nil
}

func (r *Readiness) Unready() error {
	r.IsReady = false
	return os.Remove(r.FilePath)
}

func (r *Readiness) removeReadyWhenDone(ctx context.Context) {
	<-ctx.Done()
	logger.Logger.Info("Removing readiness file")
	if err := r.Unready(); err != nil {
		logger.Logger.Errorw("Error removing readiness file", "readinessFilePath", r.FilePath, "error", err)
	}
}
