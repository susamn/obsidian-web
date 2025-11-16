# IntelliJ/GoLand Run Configurations

This directory contains pre-configured run configurations for the project.

## Available Configurations

### Test Configurations (with Coverage)

1. **All Tests with Coverage**
   - Runs all tests in the entire project
   - Generates `coverage.out` in project root
   - Use for: Full project test run

2. **Internal Tests with Coverage**
   - Runs all tests in `internal/...` packages
   - Generates `coverage-all.out`
   - Use for: Testing all internal modules

3. **Indexing Tests with Coverage**
   - Runs tests in `internal/indexing` package only
   - Generates `coverage-indexing.out`
   - Use for: Testing markdown indexing functionality

4. **Search Tests with Coverage**
   - Runs tests in `internal/search` package only
   - Generates `coverage-search.out`
   - Use for: Testing search functionality

### Application Configurations

5. **Run Main**
   - Executes the main application
   - Output directory: `bin/`
   - Use for: Running the application from IDE

## How to Use

1. Open the project in IntelliJ IDEA or GoLand
2. The run configurations will appear automatically in the run configuration dropdown (top-right)
3. Select a configuration and click the Run or Debug button
4. Coverage reports will be generated automatically and displayed in the IDE

## Viewing Coverage

After running tests with coverage:
1. IDE will automatically display coverage in the editor (green/red highlights)
2. View detailed coverage report: **Run → Show Coverage Data**
3. Coverage files are also available as `.out` files for command-line viewing:
   ```bash
   go tool cover -html=coverage.out
   ```

## Coverage Targets

- **Search Module**: 100% coverage ✅
- **Indexing Module**: 90.2% coverage ✅
- **Overall Project**: 93.5% coverage ✅

## Notes

- All test configurations use `-covermode=atomic` for accurate concurrent coverage
- Verbose mode is enabled for detailed test output
- Coverage files are git-ignored (add `*.out` to `.gitignore`)
