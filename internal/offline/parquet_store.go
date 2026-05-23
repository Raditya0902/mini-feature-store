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
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating parquet file: %w", err)
	}
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
	return nil
}

type ParquetStore struct {
	BasePath string
}

func Read(path string) ([]FeatureRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening parquet file: %w", err)
	}
	defer f.Close()

	reader := parquet.NewGenericReader[parquetRow](f)
	defer reader.Close()

	n := int(reader.NumRows())
	if n == 0 {
		return []FeatureRow{}, nil
	}

	prows := make([]parquetRow, n)
	if _, err := reader.Read(prows); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading parquet rows: %w", err)
	}

	rows := make([]FeatureRow, n)
	for i, p := range prows {
		var values map[string]any
		if err := json.Unmarshal([]byte(p.Values), &values); err != nil {
			return nil, fmt.Errorf("decoding values for row %d: %w", i, err)
		}
		rows[i] = FeatureRow{
			EntityID:         p.EntityID,
			FeatureTimestamp: time.UnixMicro(p.FeatureTimestamp).UTC(),
			Values:           values,
		}
	}
	return rows, nil
}
