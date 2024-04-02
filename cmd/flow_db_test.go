package cmd

import (
	"os"
	"testing"
)

func init() {
	err := os.MkdirAll("./output/flow", 0700)
	if err != nil {
		panic(err)
	}
}

func TestInitFlowDB(t *testing.T) {
	db := initFlowDB("test")
	if db == nil {
		t.Error("Expected database to initialize successfully")
	}
	defer os.RemoveAll("./output")
	err := createFlowsDBTable(db)
	if err != nil {
		t.Error("Unexpected error creating flows table")
	}

	// Test success case
	// {"Bytes":32}
	bs := []byte{123, 34, 66, 121, 116, 101, 115, 34, 58, 51, 50, 125}
	err = insertFlowToDB(db, bs)
	if err != nil {
		t.Errorf("Unexpected error inserting flow: %v", err)
	}

	q, err := queryDB(db, "SELECT Bytes FROM flow")
	if err != nil {
		t.Error("Unexpected error querying flow")
	}
	if len(q) != 1 {
		t.Error("Expected 1 row in flow table")
	}

	if q[0] != "32" {
		t.Error("Unexpected result from query")
	}

	// Test DB error case
	db.Close()
	err = insertFlowToDB(db, []byte("1"))
	if err == nil {
		t.Error("Expected error for closed DB")
	}
}
