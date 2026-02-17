package integration

import (
	"os"
	"testing"

	"github.com/dimitrije/nikode-api/tests/testutil"
)

// TestMain runs before all tests in this package
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

// setupTest creates a test database and returns cleanup function
func setupTest(t *testing.T) *testutil.TestDB {
	t.Helper()
	return testutil.SetupTestDB(t)
}
