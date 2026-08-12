package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/schema"
	parquet "github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputFormats(t *testing.T) {
	def := defaultOutputFormats()
	assert.True(t, def.JSON && def.SQLite && !def.Parquet)

	f, err := parseOutputFormats("")
	require.NoError(t, err)
	assert.Equal(t, def, f)

	f, err = parseOutputFormats("background=true")
	require.NoError(t, err)
	assert.Equal(t, def, f)

	f, err = parseOutputFormats("format=json,sqlite")
	require.NoError(t, err)
	assert.Equal(t, def, f)

	f, err = parseOutputFormats("format=json")
	require.NoError(t, err)
	assert.True(t, f.JSON && !f.SQLite && !f.Parquet)

	f, err = parseOutputFormats("format=parquet")
	require.NoError(t, err)
	assert.True(t, !f.JSON && !f.SQLite && f.Parquet)

	f, err = parseOutputFormats("format=json,parquet")
	require.NoError(t, err)
	assert.True(t, f.JSON && !f.SQLite && f.Parquet)

	f, err = parseOutputFormats("format=json,sqlite,parquet")
	require.NoError(t, err)
	assert.True(t, f.JSON && f.SQLite && f.Parquet)

	f, err = parseOutputFormats("background=true|format=parquet|copy=true")
	require.NoError(t, err)
	assert.True(t, f.Parquet && !f.JSON)

	// Repeatable format= flags merge
	f, err = parseOutputFormats("format=json|format=parquet")
	require.NoError(t, err)
	assert.True(t, f.JSON && f.Parquet && !f.SQLite)

	_, err = parseOutputFormats("format=unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --format token")

	_, err = parseOutputFormats("format=json,bogus")
	require.Error(t, err)

	assert.Equal(t, "json,sqlite", parseOutputFormat(""))
	assert.Equal(t, "parquet", parseOutputFormat("format=parquet"))
}

func TestParquetFlowWriter(t *testing.T) {
	tmp := t.TempDir()
	prev := outputRoot
	outputRoot = tmp
	defer func() { outputRoot = prev }()

	session := "test-parquet-session"
	w, err := newParquetFlowWriter(session, 2)
	require.NoError(t, err)

	require.NoError(t, w.Add(config.GenericMap{
		"TimeFlowStartMs":  int64(1700000000000),
		"TimeFlowEndMs":    int64(1700000001000),
		"SrcAddr":          "10.0.0.1",
		"DstAddr":          "10.0.0.2",
		"Bytes":            int64(100),
		"Packets":          int64(1),
		"SrcK8S_Namespace": "ns-a",
	}))
	assert.Equal(t, int64(0), w.BytesWritten(), "batch not full yet")

	require.NoError(t, w.Add(config.GenericMap{
		"TimeFlowStartMs":  int64(1700000002000),
		"TimeFlowEndMs":    int64(1700000003000),
		"SrcAddr":          "10.0.0.3",
		"DstAddr":          "10.0.0.4",
		"Bytes":            int64(200),
		"Packets":          int64(2),
		"SrcK8S_Namespace": "ns-b",
	}))
	assert.Greater(t, w.BytesWritten(), int64(0), "batch flushed")

	require.NoError(t, w.Flush())

	var parts []string
	err = filepath.Walk(filepath.Join(tmp, "flow", session), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".parquet" {
			parts = append(parts, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.Len(t, parts, 1)
	assert.Contains(t, parts[0], "cluster_id=cli")
	assert.Contains(t, parts[0], "part-cli-")

	data, err := os.ReadFile(parts[0])
	require.NoError(t, err)

	file, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	found := false
	for _, kv := range file.Metadata().KeyValueMetadata {
		if kv.Key == schema.ParquetVersionKey && kv.Value == schema.ParquetVersion {
			found = true
			break
		}
	}
	assert.True(t, found, "expected netobserv.parquet.version metadata")

	rows, err := parquet.Read[schema.FlowRecordV1](bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "10.0.0.1", rows[0].SrcAddr)
	assert.Equal(t, "ns-a", rows[0].SrcK8S_Namespace)
}

func TestParquetFlowWriterAddJSON(t *testing.T) {
	tmp := t.TempDir()
	prev := outputRoot
	outputRoot = tmp
	defer func() { outputRoot = prev }()

	session := "test-parquet-json"
	w, err := newParquetFlowWriter(session, 10)
	require.NoError(t, err)
	raw := []byte(`{"SrcAddr":"1.2.3.4","Bytes":42,"TimeFlowEndMs":1700000001000}`)
	require.NoError(t, w.AddJSON(raw))
	require.NoError(t, w.Flush())
	assert.Greater(t, w.BytesWritten(), int64(0))
	assert.Equal(t, int64(len(raw)), w.BytesReceived())
}

// TestParquetFlowWriterFlushOnShutdown covers the interactive CLI case: fewer
// flows than batchSize, then Close (as on collector stop). Must produce a
// .parquet file with non-zero size under the session directory.
func TestParquetFlowWriterFlushOnShutdown(t *testing.T) {
	tmp := t.TempDir()
	prev := outputRoot
	outputRoot = tmp
	defer func() { outputRoot = prev }()

	const n = 3
	session := "test-parquet-shutdown"
	w, err := newParquetFlowWriter(session, defaultParquetBatchSize)
	require.NoError(t, err)

	for i := 0; i < n; i++ {
		require.NoError(t, w.Add(config.GenericMap{
			"TimeFlowStartMs": int64(1700000000000 + i),
			"TimeFlowEndMs":   int64(1700000001000 + i),
			"SrcAddr":         "10.0.0.1",
			"DstAddr":         "10.0.0.2",
			"Bytes":           int64(100 + i),
			"Packets":         int64(1),
		}))
	}
	assert.Equal(t, int64(0), w.BytesWritten(), "nothing on disk before flush")
	assert.Equal(t, n, w.Pending())

	require.NoError(t, w.Close())
	assert.Greater(t, w.BytesWritten(), int64(0))
	assert.Equal(t, 0, w.Pending(), "Close must drain pending buffer")

	var parts []string
	err = filepath.Walk(filepath.Join(tmp, "flow", session), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".parquet" {
			parts = append(parts, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.Len(t, parts, 1, "expected one parquet part after shutdown flush")

	data, err := os.ReadFile(parts[0])
	require.NoError(t, err)
	rows, err := parquet.Read[schema.FlowRecordV1](bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	assert.Len(t, rows, n)
}

// TestParquetConcurrentAddDuringFlush ensures encode/write does not hold the mutex
// long enough to block Add (receive-loop / TUI stall every periodic flush).
func TestParquetConcurrentAddDuringFlush(t *testing.T) {
	tmp := t.TempDir()
	prev := outputRoot
	outputRoot = tmp
	defer func() { outputRoot = prev }()

	session := "test-parquet-concurrent"
	w, err := newParquetFlowWriter(session, 50)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		require.NoError(t, w.Add(config.GenericMap{
			"SrcAddr": "10.0.0.1", "DstAddr": "10.0.0.2",
			"Bytes": int64(i), "TimeFlowEndMs": int64(1700000001000 + i),
		}))
	}

	done := make(chan error, 1)
	go func() {
		done <- w.Flush()
	}()

	// Adds while flush is in flight must succeed promptly (not wait on encode I/O).
	deadline := time.After(2 * time.Second)
	for i := 0; i < 20; i++ {
		select {
		case <-deadline:
			t.Fatal("Add blocked too long during Flush")
		default:
		}
		require.NoError(t, w.Add(config.GenericMap{
			"SrcAddr": "10.0.0.3", "DstAddr": "10.0.0.4",
			"Bytes": int64(100 + i), "TimeFlowEndMs": int64(1700000002000 + i),
		}))
	}
	require.NoError(t, <-done)
	require.NoError(t, w.Close())
	assert.Greater(t, w.BytesWritten(), int64(0))
}

// TestParquetFlowWriterCloseMustFlush fails if stop skips Flush/Close while
// flows are still buffered (the bug that left empty output/flow/<capture>/).
func TestParquetFlowWriterCloseMustFlush(t *testing.T) {
	tmp := t.TempDir()
	prev := outputRoot
	outputRoot = tmp
	defer func() { outputRoot = prev }()

	session := "test-parquet-skip-flush"
	w, err := newParquetFlowWriter(session, defaultParquetBatchSize)
	require.NoError(t, err)

	require.NoError(t, w.AddJSON([]byte(
		`{"SrcAddr":"10.0.0.1","DstAddr":"10.0.0.2","Bytes":100,"TimeFlowEndMs":1700000001000}`,
	)))
	assert.Equal(t, 1, w.Pending())
	assert.Equal(t, int64(0), w.BytesWritten())

	// Simulate collector stop without Close → empty capture dir (regression).
	partsBeforeClose := listParquetParts(t, filepath.Join(tmp, "flow", session))
	assert.Empty(t, partsBeforeClose, "buffered flows must not be on disk yet")

	require.NoError(t, w.Close())

	parts := listParquetParts(t, filepath.Join(tmp, "flow", session))
	require.Len(t, parts, 1, "Close/Flush on stop must write parquet parts")
	assert.Greater(t, w.BytesWritten(), int64(0))
	assert.Greater(t, w.BytesReceived(), int64(0))
}

func TestParquetPeriodicFlushWritesEarly(t *testing.T) {
	tmp := t.TempDir()
	prev := outputRoot
	outputRoot = tmp
	defer func() { outputRoot = prev }()

	session := "test-parquet-periodic"
	w, err := newParquetFlowWriter(session, defaultParquetBatchSize)
	require.NoError(t, err)

	require.NoError(t, w.Add(config.GenericMap{
		"SrcAddr": "10.0.0.1", "DstAddr": "10.0.0.2",
		"Bytes": int64(1), "TimeFlowEndMs": int64(1700000001000),
	}))

	// Mimic the 2s periodic flusher used by startFlowCollector.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = w.Flush()
			}
		}
	}()
	defer close(done)

	require.Eventually(t, func() bool {
		return w.BytesWritten() > 0
	}, 2*time.Second, 20*time.Millisecond, "periodic Flush must write parts before batch is full")

	require.NoError(t, w.Close())
}

func listParquetParts(t *testing.T, root string) []string {
	t.Helper()
	var parts []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".parquet" {
			parts = append(parts, path)
		}
		return nil
	})
	require.NoError(t, err)
	return parts
}
