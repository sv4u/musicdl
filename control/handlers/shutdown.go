package handlers

import (
	"context"
	"time"
)

// Shutdown gracefully shuts down the handlers and stops the download service.
func (h *Handlers) Shutdown(ctx context.Context) error {
	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	// Stop download service if running
	if svcManager.IsRunning() {
		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := svcManager.StopService(shutdownCtx); err != nil {
			return err
		}
	}

	return nil
}
