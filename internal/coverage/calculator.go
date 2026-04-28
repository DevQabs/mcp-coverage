package coverage

import (
	"mcp-coverage/internal/mapping"
)

// Metrics holds top-level coverage numbers.
type Metrics struct {
	Total          int     `json:"total"`
	Mapped         int     `json:"mapped"`
	ReviewRequired int     `json:"reviewRequired"`
	Unmapped       int     `json:"unmapped"`
	CoverageRate   float64 `json:"coverageRate"` // mapped / total * 100
}

// ModuleMetrics holds per-module coverage numbers.
type ModuleMetrics struct {
	Module         string  `json:"module"`
	Total          int     `json:"total"`
	Mapped         int     `json:"mapped"`
	ReviewRequired int     `json:"reviewRequired"`
	Unmapped       int     `json:"unmapped"`
	CoverageRate   float64 `json:"coverageRate"`
}

// ControllerMetrics holds per-controller coverage numbers.
type ControllerMetrics struct {
	Controller     string  `json:"controller"`
	Module         string  `json:"module"`
	Total          int     `json:"total"`
	Mapped         int     `json:"mapped"`
	ReviewRequired int     `json:"reviewRequired"`
	Unmapped       int     `json:"unmapped"`
	CoverageRate   float64 `json:"coverageRate"`
}

// Calculate computes coverage metrics from mapping results.
func Calculate(results []mapping.MappingResult) (
	metrics Metrics,
	byModule map[string]*ModuleMetrics,
	byController map[string]*ControllerMetrics,
) {
	byModule = make(map[string]*ModuleMetrics)
	byController = make(map[string]*ControllerMetrics)

	for _, r := range results {
		metrics.Total++
		mod := r.Module
		ctrl := r.Controller

		if _, ok := byModule[mod]; !ok {
			byModule[mod] = &ModuleMetrics{Module: mod}
		}
		if _, ok := byController[ctrl]; !ok {
			byController[ctrl] = &ControllerMetrics{Controller: ctrl, Module: mod}
		}

		byModule[mod].Total++
		byController[ctrl].Total++

		switch r.MappingStatus {
		case mapping.StatusMapped:
			metrics.Mapped++
			byModule[mod].Mapped++
			byController[ctrl].Mapped++
		case mapping.StatusReviewRequired:
			metrics.ReviewRequired++
			byModule[mod].ReviewRequired++
			byController[ctrl].ReviewRequired++
		default:
			metrics.Unmapped++
			byModule[mod].Unmapped++
			byController[ctrl].Unmapped++
		}
	}

	metrics.CoverageRate = rate(metrics.Mapped, metrics.Total)
	for _, m := range byModule {
		m.CoverageRate = rate(m.Mapped, m.Total)
	}
	for _, c := range byController {
		c.CoverageRate = rate(c.Mapped, c.Total)
	}

	return
}

func rate(mapped, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(mapped) / float64(total) * 100
}
