package coverage_test

import (
	"testing"

	"mcp-coverage/internal/coverage"
	"mcp-coverage/internal/mapping"
)

func makeResults(statuses ...string) []mapping.MappingResult {
	results := make([]mapping.MappingResult, len(statuses))
	for i, s := range statuses {
		results[i] = mapping.MappingResult{MappingStatus: s}
		results[i].Module = "moduleA"
		results[i].Controller = "ControllerA"
	}
	return results
}

func TestCoverageRateFormula(t *testing.T) {
	results := makeResults(
		mapping.StatusMapped,
		mapping.StatusMapped,
		mapping.StatusReviewRequired,
		mapping.StatusUnmapped,
	)
	metrics, _, _ := coverage.Calculate(results)

	if metrics.Total != 4 {
		t.Errorf("total: want 4, got %d", metrics.Total)
	}
	if metrics.Mapped != 2 {
		t.Errorf("mapped: want 2, got %d", metrics.Mapped)
	}
	if metrics.ReviewRequired != 1 {
		t.Errorf("reviewRequired: want 1, got %d", metrics.ReviewRequired)
	}
	if metrics.Unmapped != 1 {
		t.Errorf("unmapped: want 1, got %d", metrics.Unmapped)
	}
	// coverage = 2/4 * 100 = 50.0
	if metrics.CoverageRate != 50.0 {
		t.Errorf("coverageRate: want 50.0, got %.2f", metrics.CoverageRate)
	}
}

func TestReviewRequiredNotCountedAsMapped(t *testing.T) {
	results := makeResults(
		mapping.StatusReviewRequired,
		mapping.StatusReviewRequired,
		mapping.StatusUnmapped,
	)
	metrics, _, _ := coverage.Calculate(results)

	if metrics.Mapped != 0 {
		t.Errorf("review_required must not count as mapped; got mapped=%d", metrics.Mapped)
	}
	if metrics.CoverageRate != 0 {
		t.Errorf("coverage should be 0 when only review_required and unmapped; got %.2f", metrics.CoverageRate)
	}
}

func TestAllUnmappedGivesZeroCoverage(t *testing.T) {
	results := makeResults(
		mapping.StatusUnmapped,
		mapping.StatusUnmapped,
		mapping.StatusUnmapped,
	)
	metrics, _, _ := coverage.Calculate(results)

	if metrics.CoverageRate != 0 {
		t.Errorf("expected 0%% coverage, got %.2f%%", metrics.CoverageRate)
	}
	if metrics.Unmapped != 3 {
		t.Errorf("expected 3 unmapped, got %d", metrics.Unmapped)
	}
}

func TestAllMappedGivesHundredPercentCoverage(t *testing.T) {
	results := makeResults(
		mapping.StatusMapped,
		mapping.StatusMapped,
	)
	metrics, _, _ := coverage.Calculate(results)

	if metrics.CoverageRate != 100.0 {
		t.Errorf("expected 100%% coverage, got %.2f%%", metrics.CoverageRate)
	}
}

func TestEmptyResultsGivesZeroCoverage(t *testing.T) {
	metrics, byModule, byCtrl := coverage.Calculate(nil)

	if metrics.Total != 0 || metrics.CoverageRate != 0 {
		t.Errorf("empty results should give zero metrics")
	}
	if len(byModule) != 0 || len(byCtrl) != 0 {
		t.Error("empty results should give empty module/controller maps")
	}
}

func TestModuleBreakdownIsolation(t *testing.T) {
	results := []mapping.MappingResult{
		{MappingStatus: mapping.StatusMapped},
		{MappingStatus: mapping.StatusUnmapped},
	}
	results[0].Module = "reception"
	results[0].Controller = "ReceptionController"
	results[1].Module = "lab"
	results[1].Controller = "LabOrderController"

	metrics, byModule, _ := coverage.Calculate(results)

	if metrics.Total != 2 {
		t.Errorf("total should be 2, got %d", metrics.Total)
	}
	recep, ok := byModule["reception"]
	if !ok {
		t.Fatal("module reception not found")
	}
	if recep.Mapped != 1 || recep.Unmapped != 0 {
		t.Errorf("reception: want mapped=1 unmapped=0, got mapped=%d unmapped=%d", recep.Mapped, recep.Unmapped)
	}

	lab, ok := byModule["lab"]
	if !ok {
		t.Fatal("module lab not found")
	}
	if lab.Unmapped != 1 || lab.Mapped != 0 {
		t.Errorf("lab: want mapped=0 unmapped=1, got mapped=%d unmapped=%d", lab.Mapped, lab.Unmapped)
	}
}
