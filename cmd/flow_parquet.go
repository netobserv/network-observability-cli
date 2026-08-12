package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/schema"
)

const (
	outputFormatJSON    = "json"
	outputFormatSQLite  = "sqlite"
	outputFormatParquet = "parquet"

	defaultParquetBatchSize = 5000
	parquetFlushInterval    = 2 * time.Second
	parquetStreamID         = "cli"
)

// outputFormats is a non-exclusive set of local flow sinks.
// Default (when --format is omitted) is json + sqlite.
type outputFormats struct {
	JSON    bool
	SQLite  bool
	Parquet bool
}

func (f outputFormats) String() string {
	parts := make([]string, 0, 3)
	if f.JSON {
		parts = append(parts, outputFormatJSON)
	}
	if f.SQLite {
		parts = append(parts, outputFormatSQLite)
	}
	if f.Parquet {
		parts = append(parts, outputFormatParquet)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

func (f outputFormats) empty() bool {
	return !f.JSON && !f.SQLite && !f.Parquet
}

func defaultOutputFormats() outputFormats {
	return outputFormats{JSON: true, SQLite: true}
}

// outputRoot is the directory under which flow/pcap captures are written.
// Overridable in tests.
var outputRoot = "./output"

// parseOutputFormats returns the local flows output format set from --options
// (pipe-separated). Supports comma-separated and repeated format= values, e.g.:
//
//	format=json,sqlite
//	format=parquet
//	format=json|format=parquet
//
// Default when format is omitted: json,sqlite (never parquet).
// Unknown tokens return an error.
func parseOutputFormats(opts string) (outputFormats, error) {
	var (
		found  bool
		result outputFormats
	)
	for _, part := range strings.Split(opts, "|") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "format=") {
			continue
		}
		found = true
		raw := strings.TrimPrefix(part, "format=")
		if strings.TrimSpace(raw) == "" {
			return outputFormats{}, fmt.Errorf("missing value for --format (json|sqlite|parquet)")
		}
		for _, tok := range strings.Split(raw, ",") {
			tok = strings.ToLower(strings.TrimSpace(tok))
			if tok == "" {
				continue
			}
			switch tok {
			case outputFormatJSON:
				result.JSON = true
			case outputFormatSQLite:
				result.SQLite = true
			case outputFormatParquet:
				result.Parquet = true
			default:
				return outputFormats{}, fmt.Errorf("invalid --format token %q (use json, sqlite, and/or parquet)", tok)
			}
		}
	}
	if !found || result.empty() {
		return defaultOutputFormats(), nil
	}
	return result, nil
}

// parseOutputFormat is kept for callers that only need a single primary label.
// Prefer parseOutputFormats for sink selection.
func parseOutputFormat(opts string) string {
	f, err := parseOutputFormats(opts)
	if err != nil {
		log.Warnf("%v; using json,sqlite", err)
		return defaultOutputFormats().String()
	}
	return f.String()
}

// parquetFlowWriter batches flows and writes Hive-partitioned Parquet parts under output/flow/<session>/.
type parquetFlowWriter struct {
	mu        sync.Mutex
	session   string
	baseDir   string
	batchSize int
	pending   []config.GenericMap
	seq       int64
	bytes     int64
	inbound   int64
	closed    atomic.Bool
}

func newParquetFlowWriter(session string, batchSize int) (*parquetFlowWriter, error) {
	if batchSize <= 0 {
		batchSize = defaultParquetBatchSize
	}
	base := filepath.Join(outputRoot, "flow", session)
	if err := os.MkdirAll(base, 0700); err != nil {
		return nil, fmt.Errorf("creating parquet output dir: %w", err)
	}
	return &parquetFlowWriter{
		session:   session,
		baseDir:   base,
		batchSize: batchSize,
		pending:   make([]config.GenericMap, 0, batchSize),
	}, nil
}

func (w *parquetFlowWriter) AddJSON(raw []byte) error {
	var gm config.GenericMap
	if err := json.Unmarshal(raw, &gm); err != nil {
		return err
	}
	return w.add(gm, int64(len(raw)))
}

func (w *parquetFlowWriter) Add(gm config.GenericMap) error {
	return w.add(gm, 0)
}

func (w *parquetFlowWriter) add(gm config.GenericMap, inboundBytes int64) error {
	if w.closed.Load() {
		return fmt.Errorf("parquet writer already closed")
	}
	w.mu.Lock()
	if inboundBytes > 0 {
		w.inbound += inboundBytes
	}
	w.pending = append(w.pending, gm)
	shouldFlush := len(w.pending) >= w.batchSize
	w.mu.Unlock()

	// Flush outside the lock so encode/disk I/O does not stall the receive loop
	// (and the TUI heartbeat) while a part is being written.
	if shouldFlush {
		return w.Flush()
	}
	return nil
}

// Flush writes any buffered flows to a new parquet part. Safe to call periodically and on stop.
// Encode and file I/O run without holding the writer mutex so Add can continue buffering.
func (w *parquetFlowWriter) Flush() error {
	batch, seq, ok := w.takePending()
	if !ok {
		return nil
	}

	data, err := schema.EncodeParquetBytes(batch)
	if err != nil {
		w.restoreBatch(batch)
		return fmt.Errorf("encoding parquet: %w", err)
	}

	now := time.Now().UTC()
	rel := filepath.Join(
		fmt.Sprintf("cluster_id=%s", parquetStreamID),
		fmt.Sprintf("year=%04d", now.Year()),
		fmt.Sprintf("month=%02d", now.Month()),
		fmt.Sprintf("day=%02d", now.Day()),
		fmt.Sprintf("hour=%02d", now.Hour()),
		fmt.Sprintf("part-%s-%08d.parquet", parquetStreamID, seq),
	)
	full := filepath.Join(w.baseDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		w.restoreBatch(batch)
		return err
	}
	if err := os.WriteFile(full, data, 0600); err != nil {
		w.restoreBatch(batch)
		return fmt.Errorf("writing parquet %s: %w", full, err)
	}

	w.mu.Lock()
	w.bytes += int64(len(data))
	w.mu.Unlock()
	// Debug only: Info would spam stderr every ~2s and corrupt the interactive TUI.
	log.Debugf("Wrote parquet part %s (%d bytes, %d flows)", full, len(data), len(batch))
	return nil
}

// takePending drains the buffer and reserves a part sequence number under the lock.
func (w *parquetFlowWriter) takePending() (batch []config.GenericMap, seq int64, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) == 0 {
		return nil, 0, false
	}
	batch = w.pending
	seq = w.seq
	w.seq++
	// Clear pending before encode/write so a failed flush can be retried with new flows
	// without duplicating successfully-encoded rows on the next successful write.
	w.pending = make([]config.GenericMap, 0, w.batchSize)
	return batch, seq, true
}

func (w *parquetFlowWriter) restoreBatch(batch []config.GenericMap) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(batch, w.pending...)
}

// Close flushes remaining data and marks the writer closed. Idempotent.
func (w *parquetFlowWriter) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		// Already closed — still try a final flush in case of races.
		return w.Flush()
	}
	return w.Flush()
}

func (w *parquetFlowWriter) BytesWritten() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bytes
}

// BytesReceived returns inbound JSON bytes counted before parquet parts are flushed.
func (w *parquetFlowWriter) BytesReceived() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.inbound
}

func (w *parquetFlowWriter) Pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pending)
}
