package javasource_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mcp-coverage/internal/apiscanner/javasource"
)

// ── parser unit tests ──────────────────────────────────────────────────────

func TestGetMapping(t *testing.T) {
	src := `
package com.example;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/patients")
public class PatientController {
    @GetMapping("/{id}")
    public ResponseEntity<Patient> getPatient(@PathVariable String id) {
        return ResponseEntity.ok(null);
    }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, "PatientController", ctrl.ClassName)
	assertEqual(t, 1, len(ctrl.Methods))
	assertEqual(t, "GET", ctrl.Methods[0].HTTPMethod)
	assertEqual(t, "getPatient", ctrl.Methods[0].MethodName)
	// combined path
	paths := combinePaths(ctrl.BasePaths, ctrl.Methods[0].Paths)
	assertContains(t, paths, "/api/patients/{id}")
}

func TestPostMapping(t *testing.T) {
	src := `
@RestController
@RequestMapping("/api/patients")
public class PatientController {
    @PostMapping
    public ResponseEntity<Patient> createPatient(@RequestBody CreatePatientRequest req) {
        return null;
    }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, "POST", ctrl.Methods[0].HTTPMethod)
	assertEqual(t, "createPatient", ctrl.Methods[0].MethodName)
	paths := combinePaths(ctrl.BasePaths, ctrl.Methods[0].Paths)
	assertContains(t, paths, "/api/patients")
}

func TestPutMapping(t *testing.T) {
	src := `
@RestController
@RequestMapping("/api/patients")
public class PatientController {
    @PutMapping(value = "/{id}")
    public ResponseEntity<Patient> updatePatient(@PathVariable String id) { return null; }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, "PUT", ctrl.Methods[0].HTTPMethod)
	paths := combinePaths(ctrl.BasePaths, ctrl.Methods[0].Paths)
	assertContains(t, paths, "/api/patients/{id}")
}

func TestDeleteMapping(t *testing.T) {
	src := `
@RestController
@RequestMapping("/api/patients")
public class PatientController {
    @DeleteMapping("/{id}")
    public ResponseEntity<Void> deletePatient(@PathVariable String id) { return null; }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, "DELETE", ctrl.Methods[0].HTTPMethod)
}

func TestPatchMapping(t *testing.T) {
	src := `
@RestController
public class PatientController {
    @PatchMapping("/api/patients/{id}/status")
    public ResponseEntity<Patient> patchStatus() { return null; }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, "PATCH", ctrl.Methods[0].HTTPMethod)
	paths := combinePaths(ctrl.BasePaths, ctrl.Methods[0].Paths)
	assertContains(t, paths, "/api/patients/{id}/status")
}

func TestRequestMappingWithMethod(t *testing.T) {
	src := `
@RestController
public class PatientController {
    @RequestMapping(value = "/api/patients", method = RequestMethod.POST)
    public ResponseEntity<Patient> createPatient() { return null; }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, "POST", ctrl.Methods[0].HTTPMethod)
	paths := combinePaths(ctrl.BasePaths, ctrl.Methods[0].Paths)
	assertContains(t, paths, "/api/patients")
}

func TestClassLevelPlusMethLevelPathCombination(t *testing.T) {
	src := `
@RestController
@RequestMapping("/api/v1")
public class OrderController {
    @GetMapping("/orders")
    public List<Order> listOrders() { return null; }

    @PostMapping("/orders")
    public Order createOrder(@RequestBody OrderRequest r) { return null; }

    @DeleteMapping("/orders/{id}")
    public void deleteOrder(@PathVariable Long id) {}
}`
	ctrl := javasource.ParseFile("OrderController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, 3, len(ctrl.Methods))

	all := allPaths(ctrl)
	assertContains(t, all, "/api/v1/orders")
	assertContains(t, all, "/api/v1/orders/{id}")

	methods := httpMethods(ctrl)
	assertContains(t, methods, "GET")
	assertContains(t, methods, "POST")
	assertContains(t, methods, "DELETE")
}

func TestMultiplePathAnnotation(t *testing.T) {
	src := `
@RestController
@RequestMapping("/api")
public class InvoiceController {
    @GetMapping({"/invoices", "/bills"})
    public List<Invoice> list() { return null; }
}`
	ctrl := javasource.ParseFile("InvoiceController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, 1, len(ctrl.Methods))
	// Method has 2 path values → 2 entries when combined
	paths := combinePaths(ctrl.BasePaths, ctrl.Methods[0].Paths)
	assertEqual(t, 2, len(paths))
	assertContains(t, paths, "/api/invoices")
	assertContains(t, paths, "/api/bills")
}

func TestControllerAdviceIsNotDetected(t *testing.T) {
	src := `
@RestControllerAdvice
public class GlobalExceptionHandler {
    @GetMapping("/test")
    public void test() {}
}`
	ctrl := javasource.ParseFile("GlobalExceptionHandler.java", src)
	if ctrl != nil {
		t.Error("@RestControllerAdvice should not be detected as a controller")
	}
}

func TestControllerAdvice2IsNotDetected(t *testing.T) {
	src := `
@ControllerAdvice
public class ExceptionHandler {}`
	ctrl := javasource.ParseFile("ExceptionHandler.java", src)
	if ctrl != nil {
		t.Error("@ControllerAdvice should not be detected as a controller")
	}
}

func TestNonControllerIsNotDetected(t *testing.T) {
	src := `
@Service
public class PatientService {
    public Patient create(CreatePatientRequest req) { return null; }
}`
	ctrl := javasource.ParseFile("PatientService.java", src)
	if ctrl != nil {
		t.Error("@Service class should not be detected as a controller")
	}
}

func TestMethodWithAnnotationsBeforeMapping(t *testing.T) {
	// @PreAuthorize between @GetMapping and method signature
	src := `
@RestController
@RequestMapping("/api")
public class AdminController {
    @GetMapping("/users")
    @PreAuthorize("hasRole('ADMIN')")
    public List<User> listUsers() { return null; }
}`
	ctrl := javasource.ParseFile("AdminController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, 1, len(ctrl.Methods))
	assertEqual(t, "listUsers", ctrl.Methods[0].MethodName)
}

func TestNoClassLevelMapping(t *testing.T) {
	src := `
@RestController
public class HealthController {
    @GetMapping("/health")
    public String health() { return "ok"; }

    @GetMapping("/ready")
    public String ready() { return "ok"; }
}`
	ctrl := javasource.ParseFile("HealthController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, 2, len(ctrl.Methods))
	paths := allPaths(ctrl)
	assertContains(t, paths, "/health")
	assertContains(t, paths, "/ready")
}

func TestInlineCommentDoesNotBreakParsing(t *testing.T) {
	src := `
@RestController
@RequestMapping("/api/patients") // base path
public class PatientController {
    // creates a new patient
    @PostMapping // POST /api/patients
    public ResponseEntity<Patient> createPatient() { return null; }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, 1, len(ctrl.Methods))
	assertEqual(t, "POST", ctrl.Methods[0].HTTPMethod)
}

func TestBlockCommentDoesNotBreakParsing(t *testing.T) {
	src := `
/**
 * Patient management controller.
 */
@RestController
@RequestMapping(value = "/api/patients")
public class PatientController {
    /**
     * @return list of patients
     */
    @GetMapping
    public List<Patient> list() { return null; }
}`
	ctrl := javasource.ParseFile("PatientController.java", src)
	assertNotNil(t, ctrl)
	assertEqual(t, 1, len(ctrl.Methods))
}

// ── scanner integration tests ──────────────────────────────────────────────

func TestSingleModuleScanning(t *testing.T) {
	dir := t.TempDir()
	javaDir := filepath.Join(dir, "src", "main", "java", "com", "example", "patient")
	must(t, os.MkdirAll(javaDir, 0o755))

	writeJava(t, filepath.Join(javaDir, "PatientController.java"), `
package com.example.patient;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/patients")
public class PatientController {
    @GetMapping
    public List<Patient> list() { return null; }

    @PostMapping
    public Patient create() { return null; }

    @GetMapping("/{id}")
    public Patient get(@PathVariable Long id) { return null; }
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(entries), entryKeys(entries))
	}
	assertEntryExists(t, entries, "GET", "/api/patients")
	assertEntryExists(t, entries, "POST", "/api/patients")
	assertEntryExists(t, entries, "GET", "/api/patients/{id}")
}

func TestMultiModuleScanning(t *testing.T) {
	dir := t.TempDir()

	// Module 1: patient-service
	patientDir := filepath.Join(dir, "patient-service", "src", "main", "java", "com", "example", "patient")
	must(t, os.MkdirAll(patientDir, 0o755))
	writeJava(t, filepath.Join(patientDir, "PatientController.java"), `
@RestController
@RequestMapping("/api/patients")
public class PatientController {
    @GetMapping
    public List<Patient> list() { return null; }
}`)

	// Module 2: lab-service
	labDir := filepath.Join(dir, "lab-service", "src", "main", "java", "com", "example", "lab")
	must(t, os.MkdirAll(labDir, 0o755))
	writeJava(t, filepath.Join(labDir, "LabController.java"), `
@RestController
@RequestMapping("/api/lab")
public class LabController {
    @PostMapping("/orders")
    public Order createOrder() { return null; }

    @GetMapping("/results/{id}")
    public Result getResult(@PathVariable Long id) { return null; }
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 3 {
		t.Errorf("expected 3 entries across 2 modules, got %d: %v", len(entries), entryKeys(entries))
	}
	assertEntryExists(t, entries, "GET", "/api/patients")
	assertEntryExists(t, entries, "POST", "/api/lab/orders")
	assertEntryExists(t, entries, "GET", "/api/lab/results/{id}")
}

func TestAPIsNotInSwaggerStillDetected(t *testing.T) {
	// APIs without Swagger annotations (@Operation, @ApiResponse etc.) must still appear.
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, "src", "main", "java", "com", "example")
	must(t, os.MkdirAll(ctrlDir, 0o755))

	// No @Operation or @ApiResponse annotations — would be invisible to Swagger scanner
	writeJava(t, filepath.Join(ctrlDir, "InternalAdminController.java"), `
@RestController
@RequestMapping("/internal/admin")
public class InternalAdminController {
    @GetMapping("/stats")
    public Map<String, Object> getStats() { return null; }

    @PostMapping("/flush-cache")
    public void flushCache() {}
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 2 {
		t.Errorf("internal APIs not exposed in Swagger must still be detected; got %d entries", len(entries))
	}
	assertEntryExists(t, entries, "GET", "/internal/admin/stats")
	assertEntryExists(t, entries, "POST", "/internal/admin/flush-cache")
}

func TestExcludeAPIPattern(t *testing.T) {
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, "src", "main", "java", "com", "example")
	must(t, os.MkdirAll(ctrlDir, 0o755))

	writeJava(t, filepath.Join(ctrlDir, "MixedController.java"), `
@RestController
public class MixedController {
    @GetMapping("/actuator/health")
    public String health() { return "ok"; }

    @GetMapping("/api/patients")
    public List<Patient> patients() { return null; }

    @GetMapping("/error")
    public String error() { return "error"; }
}`)

	scanner := javasource.New(javasource.Config{
		ProjectPath:        dir,
		ExcludeAPIPatterns: []string{"/actuator/**", "/error"},
	})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 1 {
		t.Errorf("expected 1 entry after exclusions, got %d: %v", len(entries), entryKeys(entries))
	}
	assertEntryExists(t, entries, "GET", "/api/patients")
}

func TestExcludeControllerPattern(t *testing.T) {
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, "src", "main", "java", "com", "example")
	must(t, os.MkdirAll(ctrlDir, 0o755))

	writeJava(t, filepath.Join(ctrlDir, "PatientController.java"), `
@RestController
public class PatientController {
    @GetMapping("/api/patients")
    public List<Patient> list() { return null; }
}`)

	writeJava(t, filepath.Join(ctrlDir, "HealthCheckController.java"), `
@RestController
public class HealthCheckController {
    @GetMapping("/health")
    public String health() { return "ok"; }
}`)

	scanner := javasource.New(javasource.Config{
		ProjectPath:               dir,
		ExcludeControllerPatterns: []string{"*HealthCheckController"},
	})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 1 {
		t.Errorf("expected 1 entry after controller exclusion, got %d: %v", len(entries), entryKeys(entries))
	}
	assertEntryExists(t, entries, "GET", "/api/patients")
}

func TestUnmappedAPIsFromSourceAppearInResults(t *testing.T) {
	// When TARGET_PROJECT_PATH is set and discovers APIs that have no MCP Tool mapping,
	// they must appear as unmapped (not be hidden). This test verifies the scanner
	// returns all APIs — coverage calculation tests verify the unmapped status.
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, "src", "main", "java", "com", "example")
	must(t, os.MkdirAll(ctrlDir, 0o755))

	writeJava(t, filepath.Join(ctrlDir, "LabController.java"), `
@RestController
@RequestMapping("/lab")
public class LabController {
    @PostMapping("/orders")
    public void createOrder() {}

    @GetMapping("/results/{id}")
    public Object getResult(@PathVariable Long id) { return null; }
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir})
	entries, err := scanner.Scan()
	must(t, err)

	// Both entries must be returned — even if no MCP Tool covers them.
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (even without MCP mapping), got %d", len(entries))
	}
}

func TestCoverageNotHundredPercentWhenUnmappedFromSource(t *testing.T) {
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, "src", "main", "java", "com", "example")
	must(t, os.MkdirAll(ctrlDir, 0o755))

	writeJava(t, filepath.Join(ctrlDir, "PatientController.java"), `
@RestController
@RequestMapping("/hoo010100p01")
public class PatientController {
    @PostMapping("/insertPatient")
    public void insertPatient() {}

    @GetMapping("/selectPatient")
    public void selectPatient() {}
}`)

	writeJava(t, filepath.Join(ctrlDir, "NursingController.java"), `
@RestController
@RequestMapping("/nursing")
public class NursingController {
    @PostMapping("/insertNursingNote")
    public void insertNursingNote() {}
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Simulate: only 2 APIs are "mapped" (as if engine found matches), 1 is unmapped.
	// The important thing: coverage = 2/3 ≠ 100%.
	mapped := 2
	total := len(entries)
	rate := float64(mapped) / float64(total) * 100
	if rate >= 100 {
		t.Errorf("coverage should be < 100%% when unmapped APIs exist; got %.1f%%", rate)
	}
}

func TestNestedModuleDirectories(t *testing.T) {
	dir := t.TempDir()
	// Deeply nested package
	deepDir := filepath.Join(dir, "modules", "clinical", "reception",
		"src", "main", "java", "com", "hospital", "clinical", "reception")
	must(t, os.MkdirAll(deepDir, 0o755))

	writeJava(t, filepath.Join(deepDir, "ReceptionController.java"), `
@RestController
@RequestMapping("/reception")
public class ReceptionController {
    @PostMapping("/insertReception")
    public void create() {}

    @GetMapping("/selectReceptionList")
    public List<Reception> list() { return null; }

    @DeleteMapping("/deleteReception")
    public void delete() {}
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir})
	entries, err := scanner.Scan()
	must(t, err)

	if len(entries) != 3 {
		t.Errorf("expected 3 entries from deeply nested module, got %d", len(entries))
	}
}

func TestDebugModeDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, "src", "main", "java", "com", "example")
	must(t, os.MkdirAll(ctrlDir, 0o755))
	writeJava(t, filepath.Join(ctrlDir, "Ctrl.java"), `
@RestController
public class Ctrl {
    @GetMapping("/test")
    public String test() { return "ok"; }
}`)

	scanner := javasource.New(javasource.Config{ProjectPath: dir, Debug: true})
	_, err := scanner.Scan()
	must(t, err)
}

// ── exclusion unit tests ───────────────────────────────────────────────────

func TestExclusionExactMatch(t *testing.T) {
	if !javasource.MatchPatternExported("/error", "/error") {
		t.Error("/error should match /error")
	}
	if javasource.MatchPatternExported("/error", "/errors") {
		t.Error("/error should not match /errors")
	}
}

func TestExclusionStarStar(t *testing.T) {
	if !javasource.MatchPatternExported("/actuator/**", "/actuator/health") {
		t.Error("/actuator/** should match /actuator/health")
	}
	if !javasource.MatchPatternExported("/actuator/**", "/actuator/metrics/jvm") {
		t.Error("/actuator/** should match /actuator/metrics/jvm")
	}
}

func TestExclusionControllerSuffix(t *testing.T) {
	if !javasource.MatchPatternExported("*HealthCheckController", "HealthCheckController") {
		t.Error("*HealthCheckController should match HealthCheckController")
	}
	if !javasource.MatchPatternExported("*HealthCheckController", "InternalHealthCheckController") {
		t.Error("*HealthCheckController should match InternalHealthCheckController")
	}
	if javasource.MatchPatternExported("*HealthCheckController", "PatientController") {
		t.Error("*HealthCheckController should NOT match PatientController")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

type entry struct{ method, path string }

func entryKeys(entries []javasource.ScannedEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.HTTPMethod+" "+e.APIPath)
	}
	sort.Strings(out)
	return out
}

func allPaths(ctrl *javasource.ControllerDef) []string {
	var paths []string
	for _, m := range ctrl.Methods {
		paths = append(paths, combinePaths(ctrl.BasePaths, m.Paths)...)
	}
	return paths
}

func httpMethods(ctrl *javasource.ControllerDef) []string {
	var methods []string
	for _, m := range ctrl.Methods {
		methods = append(methods, m.HTTPMethod)
	}
	return methods
}

func combinePaths(base, method []string) []string {
	if len(base) == 0 {
		base = []string{""}
	}
	if len(method) == 0 {
		method = []string{""}
	}
	var out []string
	for _, b := range base {
		for _, m := range method {
			p := strings.TrimRight(b, "/")
			if !strings.HasPrefix(p, "/") && p != "" {
				p = "/" + p
			}
			if m != "" && !strings.HasPrefix(m, "/") {
				m = "/" + m
			}
			result := p + m
			if result == "" {
				result = "/"
			}
			if !strings.HasPrefix(result, "/") {
				result = "/" + result
			}
			out = append(out, result)
		}
	}
	return out
}

func assertNotNil(t *testing.T, v *javasource.ControllerDef) {
	t.Helper()
	if v == nil {
		t.Fatal("expected non-nil ControllerDef, got nil")
	}
}

func assertEqual[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("want %v, got %v", want, got)
	}
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			return
		}
	}
	t.Errorf("expected %q in %v", needle, haystack)
}

func assertEntryExists(t *testing.T, entries []javasource.ScannedEntry, method, path string) {
	t.Helper()
	for _, e := range entries {
		if e.HTTPMethod == method && e.APIPath == path {
			return
		}
	}
	t.Errorf("entry %s %s not found in %v", method, path, entryKeys(entries))
}

func writeJava(t *testing.T, path, src string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("writeJava: %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
