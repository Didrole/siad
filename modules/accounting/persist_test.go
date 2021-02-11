package accounting

import (
	"reflect"
	"testing"
	"time"

	"gitlab.com/NebulousLabs/Sia/siatest/dependencies"
)

// TestPersist tests the persistence of the accounting package
func TestPersist(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Basic functionality test
	t.Run("Basic", testBasic)

	// Specific method tests
	t.Run("callThreadedPersistAccounting", testCallThreadedPersistAccounting)
	t.Run("managedUpdateAndPersistAccounting", testManagedUpdateAndPersistAccounting)
}

// testBasic tests the basic functionality of the Accounting module
func testBasic(t *testing.T) {
	// Create new accounting
	testDir := accountingTestDir(t.Name())
	fm, h, m, r, w, _ := testingParams()
	a, err := NewCustomAccounting(fm, h, m, r, w, testDir, &dependencies.AccountingDisablePersistLoop{})
	if err != nil {
		t.Fatal(err)
	}

	// Check initial persistence
	a.mu.Lock()
	initP := a.persistence
	a.mu.Unlock()
	if !reflect.DeepEqual(initP, persistence{}) {
		t.Log("initial persistence:", initP)
		t.Error("initial persistence should be empty")
	}

	// Update the persistence
	a.managedUpdateAndPersistAccounting()

	// Grab the current persistence
	a.mu.Lock()
	initP = a.persistence
	a.mu.Unlock()

	// Close accounting
	err = a.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Load Accounting
	a, err = NewCustomAccounting(fm, h, m, r, w, testDir, &dependencies.AccountingDisablePersistLoop{})
	if err != nil {
		t.Fatal(err)
	}

	// Check persistence
	a.mu.Lock()
	p := a.persistence
	a.mu.Unlock()
	if !reflect.DeepEqual(initP, p) {
		t.Log("initial persistence:", initP)
		t.Log("loaded persistence:", p)
		t.Error("loaded persistence should match persistence from before close")
	}

	// Close accounting
	err = a.Close()
	if err != nil {
		t.Fatal(err)
	}
}

// testCallThreadedPersistAccounting probes the callThreadedPersistAccounting method
func testCallThreadedPersistAccounting(t *testing.T) {
	// Initialize Accounting
	a, err := newTestAccounting(accountingTestDir(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = a.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Check that the background thread timer is working and the persistence is
	// updating
	for i := 0; i < 2; i++ {
		// Grab the current persistence
		a.mu.Lock()
		initP := a.persistence
		a.mu.Unlock()

		// Sleep
		time.Sleep(persistInterval * 2)

		// Validate the persistence was updated
		a.mu.Lock()
		p := a.persistence
		a.mu.Unlock()
		if reflect.DeepEqual(initP, p) {
			t.Fatal("persistence should be updated")
		}
	}
}

// testManagedUpdateAndPersistAccounting probes the
// managedUpdateAndPersistAccounting method
func testManagedUpdateAndPersistAccounting(t *testing.T) {
	// Initialize Accounting
	testDir := accountingTestDir(t.Name())
	fm, h, m, r, w, _ := testingParams()
	a, err := NewCustomAccounting(fm, h, m, r, w, testDir, &dependencies.AccountingDisablePersistLoop{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = a.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Grab the persistence beforehand
	a.mu.Lock()
	initP := a.persistence
	a.mu.Unlock()

	// Call managedUpdateAndPersistAccounting
	a.managedUpdateAndPersistAccounting()

	// Validate expectations
	a.mu.Lock()
	p := a.persistence
	a.mu.Unlock()
	if reflect.DeepEqual(initP, p) {
		t.Fatal("persistence should be updated")
	}
}
