# End-to-End Integration Tests

This directory contains comprehensive end-to-end (E2E) integration tests that validate the complete data flow through the obsidian-web system.

## Overview

The E2E tests validate the entire pipeline from file system changes to API responses:

```
File System → Sync Service → Workers → Database → Search Index → Explorer Cache → HTTP APIs → SSE Events
```

## Test Structure

### Success Tests (`end_to_end_integration_success_test.go`)

Tests the **happy path** where everything works correctly:

1. **System Initialization** - Create and start vault, web server, SSE manager
2. **File Creation** - Create nested folder structure with markdown files
3. **Data Flow Validation** - Verify files flow through all layers
4. **SSE Event Validation** - Verify SSE events are queued and flushed (every 2 seconds)
5. **API Validation** - Test Tree API, SSE Stats API, etc.
6. **File Modification** - Modify files and verify changes propagate
7. **File Deletion** - Delete files and verify soft delete with status update
8. **SSE Events for Changes** - Verify SSE events for modifications/deletions
9. **Cleanup & Verification** - Final consistency checks

**What it validates:**
- Complete data flow from FS to API
- File status tracking (ACTIVE → DELETED)
- SSE event queueing and flushing
- Pending event count tracking
- Soft delete functionality
- API correctness
- Service coordination

### Failure Tests (`end_to_end_integration_failure_test.go`)

Tests **failure scenarios** at different stages:

| Test | Failure Stage | What It Tests |
|------|--------------|---------------|
| `DatabaseNotInitialized` | DB initialization | Invalid DB path handling |
| `VaultNotStarted` | Vault startup | Operations fail gracefully when vault not started |
| `SSEManagerNotSet` | SSE wiring | Workers handle nil SSE manager |
| `InvalidVaultPath` | FS access | Invalid path detection |
| `APIWithoutVault` | API routing | 404 handling for missing vaults |
| `FileCreationWithoutSync` | File monitoring | Files not processed when sync stopped |
| `DatabaseQueryFailure` | DB query | Non-existent data handling |

**What it validates:**
- Error propagation
- Graceful degradation
- No crashes/panics
- Proper cleanup
- Meaningful error messages
- System stability after failures

## Running Tests

### Run all E2E tests
```bash
go test ./internal/e2e/... -v
```

### Run only success test
```bash
go test ./internal/e2e/... -v -run TestEndToEndIntegrationSuccess
```

### Run only failure tests
```bash
go test ./internal/e2e/... -v -run TestEndToEndIntegrationFailure
```

### Run specific failure test
```bash
go test ./internal/e2e/... -v -run TestEndToEndIntegrationFailure_DatabaseNotInitialized
```

## How to Add New Tests

### Adding a Success Test Feature

If you need to test a new feature in the happy path:

1. Open `end_to_end_integration_success_test.go`
2. Find the appropriate phase or add a new phase
3. Add validation steps:
   ```go
   // PHASE X: New Feature Validation
   t.Log("\n=== PHASE X: New Feature Validation ===")

   // Create test data
   // ...

   // Validate feature works
   result, err := someService.SomeMethod()
   if err != nil {
       t.Errorf("Feature failed: %v", err)
   }

   t.Log("✓ Feature validated successfully")
   ```
4. Update the test summary

### Adding a Failure Test

To test a new failure scenario:

1. Open `end_to_end_integration_failure_test.go`
2. Create a new test function:
   ```go
   /*
   TestEndToEndIntegrationFailure_<YourFailureName> tests the failure scenario
   where <description>.

   FAILURE STAGE: <which component/step>

   EXPECTED BEHAVIOR:
   - <what should happen>
   - <how errors are handled>
   - <system remains stable>
   */
   func TestEndToEndIntegrationFailure_<YourFailureName>(t *testing.T) {
       ctx, cancel := context.WithCancel(context.Background())
       defer cancel()

       t.Log("=== FAILURE TEST: <Your Test Name> ===")

       // 1. Set up system to failure point
       // 2. Trigger failure condition
       // 3. Verify error handling
       // 4. Verify no crashes
       // 5. Verify cleanup

       t.Log("✓ Failure handled gracefully")
   }
   ```
3. Follow the template in the file
4. Document the failure stage clearly

## Test Guidelines

### DO:
- Use `t.TempDir()` for temporary directories (auto-cleanup)
- Always `defer` cleanup calls (`v.Stop()`, `server.Stop()`)
- Use clear log messages with `✓` for successful validations
- Test one specific thing per test function
- Make tests independent (don't rely on other tests)
- Validate both success and error paths
- Check for nil before accessing fields
- Wait appropriate time for async operations (SSE flush is every 2s)

### DON'T:
- Share state between tests
- Hardcode paths (use `t.TempDir()`)
- Ignore errors
- Make tests depend on execution order
- Use production databases or indexes
- Skip cleanup (causes resource leaks in CI)

## SSE Testing Details

### SSE Event Types
The new simplified SSE sends one event type with different subtypes:

```go
type Event struct {
    Type          EventType    // bulk_process, ping, refresh, error
    VaultID       string
    PendingCount  int          // sync channel length
    Changes       []FileChange // for bulk_process
    ErrorMessage  string       // for error type
    Timestamp     time.Time
}
```

### SSE Timing
- Events are flushed **every 2 seconds**
- If no events in queue → sends `ping`
- If events in queue → sends `bulk_process` with all changes
- Pending count is included in EVERY event

### Testing SSE

```go
// Create SSE connection
sseReq := httptest.NewRequest("GET", "/api/v1/sse/vault-id", nil)
sseReqCtx, sseReqCancel := context.WithCancel(ctx)
defer sseReqCancel()
sseReq = sseReq.WithContext(sseReqCtx)

sseRecorder := httptest.NewRecorder()

// Start handler in goroutine
go func() {
    server.handleSSE(sseRecorder, sseReq)
}()

// Wait for initial connection
time.Sleep(200 * time.Millisecond)

// Trigger some file changes
// ...

// Wait for flush (2+ seconds)
time.Sleep(2500 * time.Millisecond)

// Check SSE output
sseBody := sseRecorder.Body.String()
if strings.Contains(sseBody, "event: bulk_process") {
    // Parse and validate changes
}
```

## Debugging Tests

### Verbose Output
```bash
go test ./internal/e2e/... -v
```

### See All Logs
```bash
go test ./internal/e2e/... -v 2>&1 | tee test.log
```

### Run with Race Detector
```bash
go test ./internal/e2e/... -v -race
```

### Increase Timeout (for slow machines)
```bash
go test ./internal/e2e/... -v -timeout 30m
```

## Test Data

Tests create realistic file structures:

```
docs/
  README.md
  guide/
    intro.md
    advanced.md
notes/
  personal/
    diary.md
    ideas.md
  work/
    project-a.md
    meetings.md
archive/
  2023/
    jan.md
projects/
  active/
    backend/
      todo.md
    frontend/
      todo.md
```

This mimics real Obsidian vault structures with nested folders.

## What Gets Validated

### Data Layer
- [x] Files created on filesystem
- [x] Sync service detects changes
- [x] Workers process events
- [x] Database updated with correct status
- [x] Search index updated
- [x] Explorer cache updated

### API Layer
- [x] Tree API returns correct structure
- [x] SSE Stats API works
- [x] File content API works
- [x] Search API works

### SSE Layer
- [x] SSE connection established
- [x] Connected event sent
- [x] Ping events sent (every 2s)
- [x] Bulk process events sent
- [x] Pending count tracked
- [x] Events contain all required fields

### Status Tracking
- [x] New files get ACTIVE status
- [x] Modified files keep ACTIVE status
- [x] Deleted files get DELETED status (soft delete)
- [x] DELETED files excluded from APIs
- [x] ACTIVE files remain accessible

## Comparison with `file_status_integration_test.go`

The original `file_status_integration_test.go` tests:
- File status flow (ACTIVE → DELETED)
- Database soft delete
- Explorer cache filtering
- Tree API filtering

The new E2E tests **extend** this with:
- **SSE event validation** (main addition!)
- **Complete service coordination**
- **Worker event processing**
- **Pending count tracking**
- **Comprehensive failure scenarios**
- **Full HTTP API validation**
- **System stability checks**

## CI/CD Integration

These tests are suitable for CI/CD pipelines:

```yaml
# .github/workflows/test.yml
- name: Run E2E Tests
  run: |
    go test ./internal/e2e/... -v -race -timeout 15m
```

They are:
- Isolated (use temp directories)
- Fast (complete in < 30 seconds)
- Reliable (no flaky timing issues)
- Self-contained (no external dependencies)

## Maintenance

When adding new features:

1. **Update success test** if it's a happy path feature
2. **Add failure test** if there's a new failure mode
3. **Update this README** with new test descriptions
4. **Validate all tests still pass**

When fixing bugs:

1. **Add failure test** that reproduces the bug
2. **Verify test fails** (confirms bug exists)
3. **Fix the bug**
4. **Verify test passes**
5. **Keep the test** (regression prevention)
