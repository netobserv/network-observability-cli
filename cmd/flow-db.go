package cmd

import (
	"database/sql"
	"encoding/json"
	"os"
	"sync"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"

	// need to import the sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

var flowsDBCmd = &cobra.Command{
	Use:   "get-flows-db",
	Short: "",
	Long:  "",
	Run:   runFlowDBQuery,
}

const flowsDB = "/tmp/flows.db"

func runFlowDBQuery(_ *cobra.Command, _ []string) {
	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	for i := range ports {
		go func(idx int) {
			defer wg.Done()
			queryFlowDB(ports[idx])
		}(i)
	}
	wg.Wait()
}

func queryFlowDB(port int) {
	os.Remove(flowsDB) // I delete the file to avoid duplicated records.
	// SQLite is a file based database.

	flowPackets := make(chan *genericmap.Flow, 100)
	collector, err := grpc.StartCollector(port, flowPackets)
	if err != nil {
		log.Error("StartCollector failed:", err.Error())
		log.Fatal(err)
	}
	go func() {
		<-utils.ExitChannel()
		close(flowPackets)
		collector.Close()
	}()

	log.Println("Creating flows.db...")
	file, err := os.Create(flowsDB) // Create SQLite file
	if err != nil {
		log.Errorf("Failed to create flows db file: %v", err.Error())
		log.Fatal(err)
	}
	file.Close()
	log.Println("flows.db created")
	// Open SQLite database
	db, err := sql.Open("sqlite3", flowsDB)
	if err != nil {
		log.Errorf("Error opening database: %v", err.Error())
		return
	}
	defer db.Close()

	// Create messages table if not exists
	err = createFlowsDBTable(db)
	if err != nil {
		log.Errorf("Error creating table: %v", err.Error())
		log.Fatal(err)
	}
	for fp := range flowPackets {
		handleFlowsDBConnection(db, fp.GenericMap.Value)
	}
}

func createFlowsDBTable(db *sql.DB) error {
	createFlowsTableSQL := `CREATE TABLE flow (
		"DnsErrno" INTEGER,
		"Dscp" INTEGER,
		"DstAddr" TEXT ,
		"DstPort" INTEGER,
		"Interface" TEXT,
		"Proto" INTEGER,
		"SrcAddr" TEXT,
		"SrcPort" INTEGER,
		"Bytes" INTEGER,
		"Packets" INTEGER,
		"PktDropLatestDropCause" TEXT,
		"PktDropBytes" INTEGER,
		"PktDropPackets" INTEGER,
		"DnsId" INTEGER,
		"DnsFlagsResponseCode" TEXT,
		"DnsLatencyMs" TIMESTAMP,
		"TimeFlowRTTNs" TIMESTAMP
	  );` // SQL Statement for Create Table

	log.Println("Create flows table...")
	statement, err := db.Prepare(createFlowsTableSQL) // Prepare SQL Statement
	if err != nil {
		if err.Error() == "table flow already exists" {
			return nil
		}
		log.Errorf("Error prepare table: %v", err.Error())
		return err
	}
	_, err = statement.Exec() // Execute SQL Statements
	if err != nil {
		if err.Error() == "table flow already exists" {
			return nil
		}
		log.Errorf("Error creating table: %v", err.Error())
		return err
	}

	log.Println("flows table created")
	return nil
}

func handleFlowsDBConnection(db *sql.DB, buf []byte) {
	flow := config.GenericMap{}

	// Unmarshal the JSON string into the flow object
	err := json.Unmarshal(buf, &flow)
	if err != nil {
		log.Errorf("Error: %s", err)
		return
	}
	// Insert message into database
	var flowSQL string
	if flow["PktDropPackets"] != 0 && flow["DnsId"] != 0 {
		flowSQL =
			`INSERT INTO flow(DnsErrno, Dscp, DstAddr, DstPort, Interface, Proto, SrcAddr, SrcPort, Bytes, Packets, PktDropLatestDropCause, PktDropBytes, PktDropPackets, DnsId, DnsFlagsResponseCode, DnsLatencyMs, TimeFlowRttNs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	} else if flow["PktDropPackets"] != 0 {
		flowSQL =
			`INSERT INTO flow(DnsErrno, Dscp, DstAddr, DstPort, Interface, Proto, SrcAddr, SrcPort, Bytes, Packets, PktDropLatestDropCause, PktDropBytes, PktDropPackets, TimeFlowRttNs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	} else if flow["DnsId"] != 0 {
		flowSQL =
			`INSERT INTO flow(DnsErrno, Dscp, DstAddr, DstPort, Interface, Proto, SrcAddr, SrcPort, Bytes, Packets, DnsId, DnsFlagsResponseCode, DnsLatencyMs, TimeFlowRttNs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	} else {
		flowSQL =
			`INSERT INTO flow(DnsErrno, Dscp, DstAddr, DstPort, Interface, Proto, SrcAddr, SrcPort, Bytes, Packets, TimeFlowRttNs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	}

	statement, err := db.Prepare(flowSQL) // Prepare statement.
	// This is good to avoid SQL injections
	if err != nil {
		log.Errorf("Error preparing SQL: %v", err.Error())
		return
	}

	if flow["PktDropLatestDropCause"] != 0 && flow["DnsId"] != 0 {
		_, err = statement.Exec(
			flow["DNSErrno"], flow["Dscp"], flow["DstAddr"], flow["DstPort"], flow["Interface"],
			flow["Proto"], flow["SrcAddr"], flow["SrcPort"], flow["Bytes"], flow["Packets"],
			flow["PktDropLatestDropCause"], flow["PktDropBytes"], flow["PktDropPackets"],
			flow["DnsId"], flow["DnsFlagsResponseCode"], flow["DnsLatencyMs"],
			flow["TimeFlowRttNs"])
	} else if flow["PktDropLatestDropCause"] != 0 {
		_, err = statement.Exec(
			flow["DNSErrno"], flow["Dscp"], flow["DstAddr"], flow["DstPort"], flow["Interface"],
			flow["Proto"], flow["SrcAddr"], flow["SrcPort"], flow["Bytes"], flow["Packets"],
			flow["PktDropLatestDropCause"], flow["PktDropBytes"], flow["PktDropPackets"],
			flow["TimeFlowRttNs"])
	} else if flow["DnsId"] != 0 {
		_, err = statement.Exec(
			flow["DNSErrno"], flow["Dscp"], flow["DstAddr"], flow["DstPort"], flow["Interface"],
			flow["Proto"], flow["SrcAddr"], flow["SrcPort"], flow["Bytes"], flow["Packets"],
			flow["DnsId"], flow["DnsFlagsResponseCode"], flow["DnsLatencyMs"],
			flow["TimeFlowRttNs"])
	} else {
		_, err = statement.Exec(
			flow["DNSErrno"], flow["Dscp"], flow["DstAddr"], flow["DstPort"], flow["Interface"],
			flow["Proto"], flow["SrcAddr"], flow["SrcPort"], flow["Bytes"], flow["Packets"],
			flow["TimeFlowRttNs"])
	}
	if err != nil {
		log.Errorf("Error inserting into database: %v", err.Error())
		return
	}
}

func QueryFlowsDB(query string) ([]string, error) {
	db, err := sql.Open("sqlite3", flowsDB)
	if err != nil {
		log.Errorf("Error opening database: %v", err.Error())
		return nil, err
	}
	defer db.Close()

	return queryDB(db, query)
}

// queryDB Function to query the database
func queryDB(db *sql.DB, query string) ([]string, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string

	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return nil, err
		}
		result = append(result, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
