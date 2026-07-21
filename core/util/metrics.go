// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"time"
)

// ProcessingMetrics contiene métricas de una operación de procesamiento
type ProcessingMetrics struct {
	OriginalSize   int
	ProcessedSize  int
	Duration       time.Duration
	ItemsProcessed int
	ErrorsFound    int
	WarningsFound  int
}

// ChangeInBytes retorna el cambio en bytes como string formateado
func (m *ProcessingMetrics) ChangeInBytes() string {
	diff := m.ProcessedSize - m.OriginalSize
	if diff == 0 {
		return "sin cambios"
	} else if diff > 0 {
		return fmt.Sprintf("+%d bytes", diff)
	} else {
		return fmt.Sprintf("%d bytes", diff)
	}
}

// SizeString retorna el tamaño formateado de manera legible
func SizeString(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d bytes", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
}

// DurationString retorna la duración en formato legible
func DurationString(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fμs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1000000)
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// ProgressTracker ayuda a trackear el progreso de operaciones largas
type ProgressTracker struct {
	logger    Logger
	stage     string
	operation string
	total     int
	current   int
}

// NewProgressTracker crea un nuevo tracker de progreso
func NewProgressTracker(logger Logger, stage, operation string, total int) *ProgressTracker {
	return &ProgressTracker{
		logger:    logger,
		stage:     stage,
		operation: operation,
		total:     total,
		current:   0,
	}
}

// Update actualiza el progreso
func (p *ProgressTracker) Update(current int) {
	p.current = current
	progress := 0
	if p.total > 0 {
		progress = (current * 100) / p.total
	}
	p.logger.Progress(p.stage, p.operation, progress)
}

// Increment incrementa el progreso en 1
func (p *ProgressTracker) Increment() {
	p.Update(p.current + 1)
}

// Finish marca el progreso como completado
func (p *ProgressTracker) Finish() {
	p.Update(p.total)
}

// OperationLogger ayuda a registrar operaciones con tiempo y métricas
type OperationLogger struct {
	logger    Logger
	category  string
	operation string
	startTime time.Time
}

// NewOperationLogger crea un nuevo logger de operación
func NewOperationLogger(logger Logger, category, operation string) *OperationLogger {
	ol := &OperationLogger{
		logger:    logger,
		category:  category,
		operation: operation,
		startTime: time.Now(),
	}

	logger.Info(category, "Iniciando %s...", operation)
	return ol
}

// Finish finaliza la operación registrando el tiempo transcurrido
func (ol *OperationLogger) Finish(metrics *ProcessingMetrics) {
	duration := time.Since(ol.startTime)
	if metrics != nil {
		metrics.Duration = duration
		ol.logger.Info(ol.category, "%s completado → %s (%s)",
			ol.operation,
			metrics.ChangeInBytes(),
			DurationString(duration))
	} else {
		ol.logger.Info(ol.category, "%s completado (%s)",
			ol.operation,
			DurationString(duration))
	}
}
