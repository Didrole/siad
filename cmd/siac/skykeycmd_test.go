package main

import (
	"fmt"
	"strings"
	"testing"

	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/Sia/node/api/client"
	"gitlab.com/NebulousLabs/Sia/skykey"
	"gitlab.com/NebulousLabs/errors"
)

// TestSkykeyCommands tests the basic functionality of the siac skykey commands
// interface. More detailed testing of the skykey manager is done in the skykey
// package.
func TestSkykeyCommands(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	groupDir := siacTestDir(t.Name())

	// Define subtests
	subTests := []subTest{
		{name: "TestDuplicateSkykeyAdd", test: testDuplicateSkykeyAdd},
		{name: "TestChangeKeyEntropyKeepName", test: testChangeKeyEntropyKeepName},
		{name: "TestAddKeyTwice", test: testAddKeyTwice},
		{name: "TestInvalidCipherType", test: testInvalidCipherType},
		{name: "TestSkykeyGet", test: testSkykeyGet},
		{name: "TestSkykeyGetUsingNameAndID", test: testSkykeyGetUsingNameAndID},
		{name: "TestSkykeyGetUsingNoNameAndNoID", test: testSkykeyGetUsingNoNameAndNoID},
		{name: "TestSkykeyListKeys", test: testSkykeyListKeys},
		{name: "TestSkykeyListKeysDoesntShowPrivateKeys", test: testSkykeyListKeysDoesntShowPrivateKeys},
		{name: "TestSkykeyListKeysAdditionalKeys", test: testSkykeyListKeysAdditionalKeys},
		{name: "TestSkykeyListKeysAdditionalKeysDoesntShowPrivateKeys", test: testSkykeyListKeysAdditionalKeysDoesntShowPrivateKeys},
	}

	// Run tests
	if err := runSubTests(t, groupDir, subTests); err != nil {
		t.Fatal(err)
	}
}

// testDuplicateSkykeyAdd tests that adding with duplicate Skykey will return
// duplicate name error.
func testDuplicateSkykeyAdd(t *testing.T, c client.Client) {
	skykeyString := "BAAAAAAAAABrZXkxAAAAAAAAAAQgAAAAAAAAADiObVg49-0juJ8udAx4qMW-TEHgDxfjA0fjJSNBuJ4a"
	err := skykeyAdd(c, skykeyString)
	if err != nil {
		t.Fatal(err)
	}

	err = skykeyAdd(c, skykeyString)
	if !strings.Contains(err.Error(), skykey.ErrSkykeyWithIDAlreadyExists.Error()) {
		t.Fatal("Expected duplicate name error but got", err)
	}
}

// testChangeKeyEntropyKeepName tests that adding with changed entropy but the
// same Skykey will return duplicate name error.
func testChangeKeyEntropyKeepName(t *testing.T, c client.Client) {
	// Change the key entropy, but keep the same name.
	var sk skykey.Skykey
	skykeyString := "BAAAAAAAAABrZXkxAAAAAAAAAAQgAAAAAAAAADiObVg49-0juJ8udAx4qMW-TEHgDxfjA0fjJSNBuJ4a"
	err := sk.FromString(skykeyString)
	if err != nil {
		t.Fatal(err)
	}
	sk.Entropy[0] ^= 1 // flip the first byte.

	skString, err := sk.ToString()
	if err != nil {
		t.Fatal(err)
	}

	// This should return a duplicate name error.
	err = skykeyAdd(c, skString)
	if !strings.Contains(err.Error(), skykey.ErrSkykeyWithNameAlreadyExists.Error()) {
		t.Fatal("Expected duplicate name error but got", err)
	}
}

// testAddKeyTwice tests that creating a Skykey with the same key name twice
// returns duplicate name error.
func testAddKeyTwice(t *testing.T, c client.Client) {
	// Check that adding same key twice returns an error.
	keyName := "createkey1"
	skykeyCipherType := "XChaCha20"
	_, err := skykeyCreate(c, keyName, skykeyCipherType)
	if err != nil {
		t.Fatal(err)
	}
	_, err = skykeyCreate(c, keyName, skykeyCipherType)
	if !strings.Contains(err.Error(), skykey.ErrSkykeyWithNameAlreadyExists.Error()) {
		t.Fatal("Expected error when creating key with same name")
	}
}

// testInvalidCipherType tests that invalid cipher types are caught.
func testInvalidCipherType(t *testing.T, c client.Client) {
	invalidSkykeyCipherType := "InvalidCipherType"
	_, err := skykeyCreate(c, "createkey2", invalidSkykeyCipherType)
	if !errors.Contains(err, crypto.ErrInvalidCipherType) {
		t.Fatal("Expected error when creating key with invalid ciphertype")
	}
}

// testSkykeyGet tests skykeyGet with known key should not return any errors.
func testSkykeyGet(t *testing.T, c client.Client) {
	keyName := "createkey testSkykeyGet"
	skykeyCipherType := "XChaCha20"
	newSkykey, err := skykeyCreate(c, keyName, skykeyCipherType)
	if err != nil {
		t.Fatal(err)
	}
	getKeyStr, err := skykeyGet(c, keyName, "")
	if err != nil {
		t.Fatal(err)
	}
	if getKeyStr != newSkykey {
		t.Fatal("Expected keys to match")
	}
}

// testSkykeyGetUsingNameAndID tests using both name and id params should return an
// error.
func testSkykeyGetUsingNameAndID(t *testing.T, c client.Client) {
	_, err := skykeyGet(c, "name", "id")
	if err == nil {
		t.Fatal("Expected error when using both name and id")
	}
}

// testSkykeyGetUsingNoNameAndNoID test using neither name or id param should return an
// error.
func testSkykeyGetUsingNoNameAndNoID(t *testing.T, c client.Client) {
	_, err := skykeyGet(c, "", "")
	if err == nil {
		t.Fatal("Expected error when using neither name or id params")
	}
}

// testSkykeyListKeys tests that skykeyListKeys shows key names, ids and keys
func testSkykeyListKeys(t *testing.T, c client.Client) {
	nKeys := 3
	nExtraLines := 3
	keyStrings := make([]string, nKeys)
	keyNames := make([]string, nKeys)
	keyIDs := make([]string, nKeys)

	initSkykeyData(t, c, keyStrings, keyNames, keyIDs)

	keyListString, err := skykeyListKeys(c, true)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < nKeys; i++ {
		if !strings.Contains(keyListString, keyNames[i]) {
			t.Log(keyListString)
			t.Fatal("Missing key name!", i)
		}
		if !strings.Contains(keyListString, keyIDs[i]) {
			t.Log(keyListString)
			t.Fatal("Missing id!", i)
		}
		if !strings.Contains(keyListString, keyStrings[i]) {
			t.Log(keyListString)
			t.Fatal("Missing key!", i)
		}
	}
	keyList := strings.Split(keyListString, "\n")
	if len(keyList) != nKeys+nExtraLines {
		t.Log(keyListString)
		t.Fatalf("Unexpected number of lines/keys %d, Expected %d", len(keyList), nKeys+nExtraLines)
	}
}

// testKskykeyListKeysDoesntShowPrivateKeys tests that skykeyListKeys shows key names, ids and
// doesn't show private keys
func testSkykeyListKeysDoesntShowPrivateKeys(t *testing.T, c client.Client) {
	nKeys := 3
	nExtraLines := 3
	keyStrings := make([]string, nKeys)
	keyNames := make([]string, nKeys)
	keyIDs := make([]string, nKeys)

	initSkykeyData(t, c, keyStrings, keyNames, keyIDs)

	keyListString, err := skykeyListKeys(c, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < nKeys; i++ {
		if !strings.Contains(keyListString, keyNames[i]) {
			t.Log(keyListString)
			t.Fatal("Missing key name!", i)
		}
		if !strings.Contains(keyListString, keyIDs[i]) {
			t.Log(keyListString)
			t.Fatal("Missing id!", i)
		}
		if strings.Contains(keyListString, keyStrings[i]) {
			t.Log(keyListString)
			t.Fatal("Found key!", i)
		}
	}
	keyList := strings.Split(keyListString, "\n")
	if len(keyList) != nKeys+nExtraLines {
		t.Fatal("Unexpected number of lines/keys", len(keyList))
	}
}

// testSkykeyListKeysAdditionalKeys tests that after creating additional keys,
// skykeyListKeys shows all key names, ids and keys
func testSkykeyListKeysAdditionalKeys(t *testing.T, c client.Client) {
	nExtraKeys := 10
	nKeys := 3 + nExtraKeys
	nExtraLines := 3
	keyStrings := make([]string, nKeys)
	keyNames := make([]string, nKeys)
	keyIDs := make([]string, nKeys)
	skykeyCipherType := "XChaCha20"

	initSkykeyData(t, c, keyStrings, keyNames, keyIDs)

	// Add extra keys
	for i := 0; i < nExtraKeys; i++ {
		nextName := fmt.Sprintf("extrakey-%d", i)
		keyNames = append(keyNames, nextName)
		nextSkStr, err := skykeyCreate(c, nextName, skykeyCipherType)
		if err != nil {
			t.Fatal(err)
		}
		var nextSkykey skykey.Skykey
		err = nextSkykey.FromString(nextSkStr)
		if err != nil {
			t.Fatal(err)
		}
		keyIDs = append(keyIDs, nextSkykey.ID().ToString())
		keyStrings = append(keyStrings, nextSkStr)
	}

	// Check that all the key names and key data is there.
	keyListString, err := skykeyListKeys(c, true)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < nKeys; i++ {
		if !strings.Contains(keyListString, keyNames[i]) {
			t.Log(keyListString)
			t.Fatal("Missing key name!", i)
		}
		if !strings.Contains(keyListString, keyIDs[i]) {
			t.Log(keyListString)
			t.Fatal("Missing id!", i)
		}
		if !strings.Contains(keyListString, keyStrings[i]) {
			t.Log(keyListString)
			t.Fatal("Missing key!", i)
		}
	}
	keyList := strings.Split(keyListString, "\n")
	if len(keyList) != nKeys+nExtraLines {
		t.Fatal("Unpected number of lines/keys", len(keyList))
	}
}

// testSkykeyListKeysAdditionalKeysDoesntShowPrivateKeys tests that after creating additional keys,
// skykeyListKeys shows all key names, ids and doesn't show private keys
func testSkykeyListKeysAdditionalKeysDoesntShowPrivateKeys(t *testing.T, c client.Client) {
	nExtraKeys := 10
	nPrevKeys := 3
	nKeys := nPrevKeys + nExtraKeys
	nExtraLines := 3
	keyStrings := make([]string, nPrevKeys)
	keyNames := make([]string, nPrevKeys)
	keyIDs := make([]string, nPrevKeys)

	initSkykeyData(t, c, keyStrings, keyNames, keyIDs)

	// Get extra keys
	for i := 0; i < nExtraKeys; i++ {
		nextName := fmt.Sprintf("extrakey-%d", i)
		keyNames = append(keyNames, nextName)
		nextSkStr, err := skykeyGet(c, nextName, "")
		if err != nil {
			t.Fatal(err)
		}
		var nextSkykey skykey.Skykey
		err = nextSkykey.FromString(nextSkStr)
		if err != nil {
			t.Fatal(err)
		}
		keyIDs = append(keyIDs, nextSkykey.ID().ToString())
		keyStrings = append(keyStrings, nextSkStr)
	}

	// Make sure key data isn't shown but otherwise the same checks pass.
	keyListString, err := skykeyListKeys(c, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < nKeys; i++ {
		if !strings.Contains(keyListString, keyNames[i]) {
			t.Log(keyListString)
			t.Fatal("Missing key name!", i)
		}
		if !strings.Contains(keyListString, keyIDs[i]) {
			t.Log(keyListString)
			t.Fatal("Missing id!", i)
		}
		if strings.Contains(keyListString, keyStrings[i]) {
			t.Log(keyListString)
			t.Fatal("Found key!", i)
		}
	}
	keyList := strings.Split(keyListString, "\n")
	if len(keyList) != nKeys+nExtraLines {
		t.Fatal("Unexpected number of lines/keys", len(keyList))
	}
}

// initSkykeyData initializes keyStrings, keyNames, keyIDS slices with existing Skykey data
func initSkykeyData(t *testing.T, c client.Client, keyStrings, keyNames, keyIDs []string) {
	keyName1 := "createkey1"
	keyName2 := "createkey testSkykeyGet"
	testSkykeyString := "BAAAAAAAAABrZXkxAAAAAAAAAAQgAAAAAAAAADiObVg49-0juJ8udAx4qMW-TEHgDxfjA0fjJSNBuJ4a"

	getKeyStr1, err := skykeyGet(c, keyName1, "")
	if err != nil {
		t.Fatal(err)
	}

	getKeyStr2, err := skykeyGet(c, keyName2, "")
	if err != nil {
		t.Fatal(err)
	}

	keyNames[0] = "key1"
	keyNames[1] = keyName1
	keyNames[2] = keyName2
	keyStrings[0] = testSkykeyString
	keyStrings[1] = getKeyStr1
	keyStrings[2] = getKeyStr2

	var sk skykey.Skykey
	err = sk.FromString(testSkykeyString)
	if err != nil {
		t.Fatal(err)
	}
	keyIDs[0] = sk.ID().ToString()

	err = sk.FromString(getKeyStr1)
	if err != nil {
		t.Fatal(err)
	}
	keyIDs[1] = sk.ID().ToString()

	err = sk.FromString(getKeyStr2)
	if err != nil {
		t.Fatal(err)
	}
	keyIDs[2] = sk.ID().ToString()
}
