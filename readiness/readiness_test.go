package readiness

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

var testDir = "tmpTest"
var readinessFilePath = testDir + "/test-ready"

func TestReady(t *testing.T) {
	err := setupTestDirectory()
	if err != nil {
		t.Error("Error setting up tmp test directory", err)
		return
	}

	t.Run("Test readiness file is produced", testShowReadinessFile)
	t.Run("Test readiness file is removed", testRemoveReadinessFile)

	err = os.RemoveAll(testDir)
	if err != nil {
		assert.NoError(t, err)
	}

}

func testShowReadinessFile(t *testing.T) {
	// Given
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	readiness := New(timeout, readinessFilePath)

	// When
	err := readiness.Ready()
	if err != nil {
		assert.NoError(t, err)
		return
	}

	// Then
	// Check the readiness file is created
	_, err = os.Stat(readinessFilePath)
	if !assert.NoError(t, err, "Readiness file was not present") {
		return
	}

	// Check the readiness object is ready
	assert.True(t, readiness.IsReady)

}
func testRemoveReadinessFile(t *testing.T) {

	// Given
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	readiness := New(timeout, readinessFilePath)

	// When
	err := readiness.Ready()
	if err != nil {
		assert.NoError(t, err)
		return
	}

	// Trigger the cancel which should result in the file being removed
	removeCtx, readinessCancel := context.WithCancel(timeout)
	readinessCancel()

	readiness.removeReadyWhenDone(removeCtx)

	for {
		// Check if readiness file has been removed
		if _, err = os.Stat(readinessFilePath); err != nil {
			// Succeed if the file does not now exist
			if os.IsNotExist(err) {
				break
			}
			// Error the test on any other error
			assert.NoError(t, err)
			return
		}

		// Fail the test if it times out before the file is removed
		select {
		case <-timeout.Done():
			assert.Fail(t, "Test timed out waiting for file cleanup")
			return
		default:
		}

	}
	// Check the readiness object is no longer ready
	assert.False(t, readiness.IsReady)
	fmt.Println(readiness)
}

func setupTestDirectory() error {
	err := os.RemoveAll(testDir)
	if err != nil {
		return err
	}
	return os.Mkdir(testDir, 0700)
}
