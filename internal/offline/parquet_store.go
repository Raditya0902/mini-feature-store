package offline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/parquet-go/parquet-go"
)

type FeatureRow struct {
	EntityID         string
	FeatureTimestamp time.Time
	Values           map[string]any
}

type parquetRow struct {
	EntityID         string `parquet:"entity_id"`
	FeatureTimestamp int64  `parquet:"feature_timestamp"` // Unix microseconds
	Values           string `parquet:"values"`            // JSON-encoded map[string]any
}

func Write(path string, rows []FeatureRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".features-*.parquet")
	if err != nil {
		return fmt.Errorf("creating parquet file: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	writer := parquet.NewGenericWriter[parquetRow](f)

	prows := make([]parquetRow, len(rows))
	for i, r := range rows {
		encoded, err := json.Marshal(r.Values)
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("encoding values for row %d: %w", i, err)
		}
		prows[i] = parquetRow{
			EntityID:         r.EntityID,
			FeatureTimestamp: r.FeatureTimestamp.UnixMicro(),
			Values:           string(encoded),
		}
	}

	if _, err := writer.Write(prows); err != nil {
		_ = writer.Close()
		return fmt.Errorf("writing parquet rows: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing parquet writer: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing parquet file: %w", err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("publishing parquet file: %w", err)
	}
	return nil
}

type ParquetStore struct {
	BasePath string
}

func Read(path string) ([]FeatureRow, error) {
	rows := []FeatureRow{}
	err := Scan(path, func(row FeatureRow) {
		rows = append(rows, row)
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Scan visits feature rows in file order in bounded batches.
func Scan(path string, visit func(FeatureRow)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening parquet file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("checking parquet file: %w", err)
	}
	file, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return fmt.Errorf("opening parquet data: %w", err)
	}
	reader := parquet.NewGenericReader[parquetRow](file)
	defer reader.Close()

	batch := make([]parquetRow, 1024)
	for offset := 0; ; {
		n, err := reader.Read(batch)
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("reading parquet rows: %w", err)
		}
		for i, row := range batch[:n] {
			var values map[string]any
			if err := json.Unmarshal([]byte(row.Values), &values); err != nil {
				return fmt.Errorf("decoding values for row %d: %w", offset+i, err)
			}
			visit(FeatureRow{
				EntityID:         row.EntityID,
				FeatureTimestamp: time.UnixMicro(row.FeatureTimestamp).UTC(),
				Values:           values,
			})
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		offset += n
	}
}
