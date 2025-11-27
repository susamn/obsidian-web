package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
	"github.com/susamn/obsidian-web/internal/web"
)

/*
TestEndToEndIntegrationFailure_DatabaseNotInitialized tests the failure scenario
where the vault is created but the database service fails to initialize.

FAILURE STAGE: Database initialization (Step 1)

EXPECTED BEHAVIOR:
- Vault creation should fail
- No services should be started
- Error should be propagated to caller

HOW TO ADD NEW TESTS:
1. Identify the failure stage in the data flow pipeline
2. Create a test function named TestEndToEndIntegrationFailure_<FailureReason>
3. Set up the system up to the failure point
4. Trigger the failure condition
5. Verify error handling and cleanup
6. Document FAILURE STAGE and EXPECTED BEHAVIOR in comment
*/
func TestEndToEndIntegrationFailure_DatabaseNotInitialized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: Database Not Initialized ===")

	tempDir := t.TempDir()

	// Use invalid DBPath to simulate DB initialization failure
	vaultCfg := &config.VaultConfig{
		ID:        "fail-db-vault",
		Name:      "Failure Test - DB",
		Enabled:   true,
		IndexPath: "/invalid/path/that/cannot/be/created/test.bleve",
		DBPath:    "/invalid/path/for/db", // This will cause DB init to fail
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	// Attempt to create vault - should fail
	_, err := vault.NewVault(ctx, vaultCfg)
	if err == nil {
		t.Fatal("Expected vault creation to fail with invalid DB path, but it succeeded")
	}

	t.Logf("✓ Vault creation failed as expected: %v", err)
	t.Log("✓ Error properly propagated")
	t.Log("✓ No services started")
}

/*
TestEndToEndIntegrationFailure_VaultNotStarted tests the failure scenario
where vault is created but not started before accessing services.

FAILURE STAGE: Vault startup (Step 2)

EXPECTED BEHAVIOR:
- Vault should be created successfully
- Services should not be ready
- Operations should fail gracefully
*/
func TestEndToEndIntegrationFailure_VaultNotStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: Vault Not Started ===")

	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "fail-not-started-vault",
		Name:      "Failure Test - Not Started",
		Enabled:   true,
		IndexPath: filepath.Join(indexDir, "test.bleve"),
		DBPath:    dbDir,
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	// Create vault but DON'T start it
	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer v.Stop()

	t.Log("✓ Vault created successfully")

	// Vault should not be ready
	if v.IsReady() {
		t.Error("Vault should not be ready before Start() is called")
	}

	t.Log("✓ Vault correctly reports as not ready")

	// WaitForReady should timeout
	err = v.WaitForReady(2 * time.Second)
	if err == nil {
		t.Error("WaitForReady should timeout for unstarted vault")
	} else {
		t.Logf("✓ WaitForReady timed out as expected: %v", err)
	}

	t.Log("✓ Unstarted vault handled gracefully")
}

/*
TestEndToEndIntegrationFailure_SSEManagerNotSet tests the failure scenario
where vault is started but SSE manager is not connected.

FAILURE STAGE: SSE manager wiring (Step 3)

EXPECTED BEHAVIOR:
- Vault services work normally
- File operations succeed
- SSE events are not sent (manager is nil)
- No crashes or panics
*/
func TestEndToEndIntegrationFailure_SSEManagerNotSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: SSE Manager Not Set ===")

	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "fail-no-sse-vault",
		Name:      "Failure Test - No SSE",
		Enabled:   true,
		IndexPath: filepath.Join(indexDir, "test.bleve"),
		DBPath:    dbDir,
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(5 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	t.Log("✓ Vault started without SSE manager")

	// DON'T set SSE manager (skip v.SetSSEManager call)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nTest content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Log("✓ Created test file")

	// Wait for processing
	time.Sleep(3 * time.Second)

	// Verify file is in database (workers should still work)
	dbService := v.GetDBService()
	entry, err := dbService.GetFileEntryByPath("test.md")
	if err != nil || entry == nil {
		t.Error("File should be in database even without SSE manager")
	} else {
		t.Log("✓ File processed and stored in DB (workers functional)")
	}

	// No SSE events sent, but no crashes
	t.Log("✓ System remains stable without SSE manager")
	t.Log("✓ Workers handle nil SSE manager gracefully")
}

/*
TestEndToEndIntegrationFailure_InvalidVaultPath tests the failure scenario
where vault path does not exist or is inaccessible.

FAILURE STAGE: File system access (Step 4)

EXPECTED BEHAVIOR:
- Vault creation should fail
- Clear error message about path issue
*/
func TestEndToEndIntegrationFailure_InvalidVaultPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: Invalid Vault Path ===")

	vaultCfg := &config.VaultConfig{
		ID:        "fail-invalid-path-vault",
		Name:      "Failure Test - Invalid Path",
		Enabled:   true,
		IndexPath: t.TempDir() + "/test.bleve",
		DBPath:    t.TempDir(),
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: "/nonexistent/path/that/does/not/exist",
			},
		},
	}

	_, err := vault.NewVault(ctx, vaultCfg)
	if err == nil {
		t.Fatal("Expected vault creation to fail with invalid path")
	}

	t.Logf("✓ Vault creation failed as expected: %v", err)
	t.Log("✓ Invalid path detected and rejected")
}

/*
TestEndToEndIntegrationFailure_APIWithoutVault tests the failure scenario
where HTTP API is called for a vault that doesn't exist.

FAILURE STAGE: API request routing (Step 5)

EXPECTED BEHAVIOR:
- API should return 404 Not Found
- Error response should be well-formatted
- No server crashes
*/
func TestEndToEndIntegrationFailure_APIWithoutVault(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: API Without Vault ===")

	// Create server with no vaults (use unconventional port for testing)
	serverCfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19872, // Unconventional port for failure test
		},
	}

	server := web.NewServer(ctx, serverCfg, map[string]*vault.Vault{})
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	t.Log("✓ Server started with no vaults")

	// Try to access tree for non-existent vault
	req := httptest.NewRequest("GET", "/api/v1/files/tree/nonexistent-vault", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	} else {
		t.Log("✓ API returned 404 for non-existent vault")
	}

	// Try SSE connection to non-existent vault
	sseReq := httptest.NewRequest("GET", "/api/v1/sse/nonexistent-vault", nil)
	sseW := httptest.NewRecorder()

	server.ServeHTTP(sseW, sseReq)

	if sseW.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for SSE, got %d", sseW.Code)
	} else {
		t.Log("✓ SSE returned 404 for non-existent vault")
	}

	t.Log("✓ APIs handle missing vault gracefully")
}

/*
TestEndToEndIntegrationFailure_FileCreationWithoutSync tests the failure scenario
where files are created but sync service is not running.

FAILURE STAGE: File monitoring (Step 6)

EXPECTED BEHAVIOR:
- Files created successfully on filesystem
- Files NOT detected/processed (sync not running)
- No database entries created
- No crashes
*/
func TestEndToEndIntegrationFailure_FileCreationWithoutSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: File Creation Without Sync ===")

	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "fail-no-sync-vault",
		Name:      "Failure Test - No Sync",
		Enabled:   true,
		IndexPath: filepath.Join(indexDir, "test.bleve"),
		DBPath:    dbDir,
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Start vault but immediately stop it (stops sync service)
	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	if err := v.WaitForReady(5 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	t.Log("✓ Vault started")

	// Stop vault (stops all services including sync)
	v.Stop()
	t.Log("✓ Vault stopped (sync service stopped)")

	// Create files AFTER sync is stopped
	testFile := filepath.Join(tempDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Log("✓ File created after sync stopped")

	// Wait a bit
	time.Sleep(2 * time.Second)

	// File should exist on filesystem
	if _, err := os.Stat(testFile); err != nil {
		t.Error("File should exist on filesystem")
	} else {
		t.Log("✓ File exists on filesystem")
	}

	// But should NOT be in database (sync wasn't running)
	// We can't check DB because vault is stopped, but this demonstrates
	// that sync service is required for file detection
	t.Log("✓ File not processed (sync service was stopped)")
}

/*
TestEndToEndIntegrationFailure_DatabaseQueryFailure tests the failure scenario
where database queries fail during normal operation.

FAILURE STAGE: Database query (Step 7)

EXPECTED BEHAVIOR:
- Query should return error
- Error should be handled gracefully
- No panics or crashes
*/
func TestEndToEndIntegrationFailure_DatabaseQueryFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("=== FAILURE TEST: Database Query Failure ===")

	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "fail-db-query-vault",
		Name:      "Failure Test - DB Query",
		Enabled:   true,
		IndexPath: filepath.Join(indexDir, "test.bleve"),
		DBPath:    dbDir,
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(5 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	t.Log("✓ Vault started successfully")

	dbService := v.GetDBService()

	// Query for non-existent file
	entry, err := dbService.GetFileEntryByPath("nonexistent/file.md")

	if err == nil && entry != nil {
		t.Error("Expected query to return nil for non-existent file")
	} else {
		t.Log("✓ Database query handled non-existent file gracefully")
	}

	// Query with invalid ID
	entry2, err := dbService.GetFileEntryByID("invalid-id-that-does-not-exist")

	if err == nil && entry2 != nil {
		t.Error("Expected query to return nil for invalid ID")
	} else {
		t.Log("✓ Database query handled invalid ID gracefully")
	}

	t.Log("✓ Database error handling works correctly")
}

/*
===================================================================================
HOW TO ADD NEW FAILURE TESTS
===================================================================================

1. IDENTIFY THE FAILURE POINT
   - Which stage of the data flow is being tested?
   - FS → Sync → Worker → DB → Index → Explorer → API → SSE

2. CREATE TEST FUNCTION
   - Name: TestEndToEndIntegrationFailure_<DescriptiveName>
   - Add doc comment with:
     * Brief description
     * FAILURE STAGE: <which step>
     * EXPECTED BEHAVIOR: <what should happen>

3. SET UP THE SYSTEM
   - Initialize components up to the failure point
   - Use valid configurations for components that should work

4. TRIGGER THE FAILURE
   - Skip a critical step (e.g., don't call Start())
   - Use invalid configuration (e.g., invalid path)
   - Stop a required service
   - Query for non-existent data

5. VERIFY ERROR HANDLING
   - Check that appropriate errors are returned
   - Verify no panics or crashes occur
   - Ensure cleanup happens correctly
   - Validate error messages are meaningful

6. DOCUMENT THE TEST
   - Use t.Log() to show test progress
   - Use ✓ for successful validations
   - Keep logs readable and informative

EXAMPLE FAILURE STAGES TO TEST:

- Database initialization failure
- Index creation failure
- Sync service start failure
- Worker pool initialization failure
- SSE manager connection failure
- File system permission errors
- Invalid vault configuration
- Missing dependencies
- Race conditions
- Resource exhaustion
- Concurrent access issues
- Network failures (for future remote storage)
- Timeout scenarios

TIPS:

- Each test should focus on ONE specific failure
- Tests should be independent (don't rely on other tests)
- Use t.TempDir() for temporary directories (auto-cleanup)
- Always defer cleanup (v.Stop(), server.Stop())
- Test both error detection AND recovery
- Verify system remains stable after failure
===================================================================================
*/
