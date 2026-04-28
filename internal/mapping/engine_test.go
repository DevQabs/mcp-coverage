package mapping_test

import (
	"testing"

	"mcp-coverage/internal/apiscanner"
	"mcp-coverage/internal/mapping"
	"mcp-coverage/internal/mcpclient"
)

func tools(names ...string) []mcpclient.ToolEntry {
	out := make([]mcpclient.ToolEntry, len(names))
	for i, n := range names {
		out[i] = mcpclient.ToolEntry{Name: n}
	}
	return out
}

func apis(entries ...apiscanner.APIEntry) []apiscanner.APIEntry { return entries }

func TestUnmappedAPIAppearsAsUnmapped(t *testing.T) {
	engine, err := mapping.NewEngine("testdata", tools("create_patient"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	input := apis(apiscanner.APIEntry{
		Module:     "lab",
		Controller: "LabOrderController",
		HTTPMethod: "POST",
		APIPath:    "/lab/insertLabOrder",
		MethodName: "insertLabOrder",
	})

	results := engine.Map(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.MappingStatus != mapping.StatusUnmapped {
		t.Errorf("expected status=unmapped, got %q", r.MappingStatus)
	}
	if r.MCPToolName != "" {
		t.Errorf("expected empty mcpToolName, got %q", r.MCPToolName)
	}
}

func TestCoverageIsNotHundredPercentWhenUnmappedExist(t *testing.T) {
	engine, err := mapping.NewEngine("testdata", tools("create_patient", "search_patient"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	input := apis(
		apiscanner.APIEntry{HTTPMethod: "POST", APIPath: "/hoo010100p01/insertPatient", Controller: "PatientController", MethodName: "insertPatient"},
		apiscanner.APIEntry{HTTPMethod: "GET", APIPath: "/lab/selectLabResult", Controller: "LabOrderController", MethodName: "selectLabResult"},
		apiscanner.APIEntry{HTTPMethod: "POST", APIPath: "/nursing/insertNursingNote", Controller: "NursingController", MethodName: "insertNursingNote"},
	)

	results := engine.Map(input)

	var mapped, unmapped int
	for _, r := range results {
		switch r.MappingStatus {
		case mapping.StatusMapped:
			mapped++
		case mapping.StatusUnmapped:
			unmapped++
		}
	}

	if mapped == len(input) {
		t.Errorf("expected some unmapped APIs, but all %d were mapped", len(input))
	}
	if unmapped == 0 {
		t.Error("expected at least one unmapped API")
	}

	coverageRate := float64(mapped) / float64(len(input)) * 100
	if coverageRate >= 100 {
		t.Errorf("coverage should be < 100%%, got %.1f%%", coverageRate)
	}
}

func TestExplicitMetadataTakesPriorityOverSimilarity(t *testing.T) {
	engine, err := mapping.NewEngine("testdata", tools("create_patient", "some_other_patient_tool"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	input := apis(apiscanner.APIEntry{
		HTTPMethod: "POST",
		APIPath:    "/hoo010100p01/insertPatient",
		Controller: "PatientController",
		MethodName: "insertPatient",
	})

	results := engine.Map(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.MCPToolName != "create_patient" {
		t.Errorf("expected create_patient (explicit metadata), got %q", r.MCPToolName)
	}
	if r.Remark != "explicit metadata" {
		t.Errorf("expected remark='explicit metadata', got %q", r.Remark)
	}
}

func TestMultiAPIToolMapsAllDeclaredPaths(t *testing.T) {
	engine, err := mapping.NewEngine("testdata", tools("search_patient"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	input := apis(
		apiscanner.APIEntry{HTTPMethod: "GET", APIPath: "/hoo010100p01/selectPatient"},
		apiscanner.APIEntry{HTTPMethod: "GET", APIPath: "/reception/selectCompletePatientList"},
	)

	results := engine.Map(input)
	for _, r := range results {
		if r.MappingStatus != mapping.StatusMapped {
			t.Errorf("path %s should be mapped via search_patient, got status=%s", r.APIPath, r.MappingStatus)
		}
		if r.MCPToolName != "search_patient" {
			t.Errorf("expected mcpToolName=search_patient for %s, got %q", r.APIPath, r.MCPToolName)
		}
	}
}

func TestNoToolsProducesAllUnmapped(t *testing.T) {
	engine, err := mapping.NewEngine("testdata", tools()) // no tools at all
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	input := apis(
		apiscanner.APIEntry{HTTPMethod: "POST", APIPath: "/hoo010100p01/insertPatient", Controller: "PatientController", MethodName: "insertPatient"},
		apiscanner.APIEntry{HTTPMethod: "GET", APIPath: "/oneai/doctors", Controller: "OneAIController", MethodName: "getDoctors"},
	)

	results := engine.Map(input)
	for _, r := range results {
		if r.MappingStatus != mapping.StatusUnmapped {
			t.Errorf("expected unmapped (no tools registered), got status=%s for %s", r.MappingStatus, r.APIPath)
		}
	}
}

func TestUnmappedApisAreIncludedInResultSlice(t *testing.T) {
	engine, err := mapping.NewEngine("testdata", tools("create_patient"))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	input := apis(
		apiscanner.APIEntry{HTTPMethod: "POST", APIPath: "/hoo010100p01/insertPatient", Controller: "PatientController", MethodName: "insertPatient"},
		apiscanner.APIEntry{HTTPMethod: "POST", APIPath: "/lab/insertLabOrder", Controller: "LabOrderController", MethodName: "insertLabOrder"},
		apiscanner.APIEntry{HTTPMethod: "POST", APIPath: "/nursing/insertNursingNote", Controller: "NursingController", MethodName: "insertNursingNote"},
	)

	results := engine.Map(input)

	if len(results) != len(input) {
		t.Errorf("result count %d != input count %d: unmapped APIs must be included", len(results), len(input))
	}

	unmappedPaths := map[string]bool{}
	for _, r := range results {
		if r.MappingStatus == mapping.StatusUnmapped {
			unmappedPaths[r.APIPath] = true
		}
	}
	if !unmappedPaths["/lab/insertLabOrder"] {
		t.Error("/lab/insertLabOrder should appear as unmapped in results")
	}
	if !unmappedPaths["/nursing/insertNursingNote"] {
		t.Error("/nursing/insertNursingNote should appear as unmapped in results")
	}
}
