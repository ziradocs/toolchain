// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// ChartNormalizationReceipt is an external audit record, never part of the effective AST.
// Hashes cover the semantic AST JSON and exact raw JSON string respectively.
type ChartNormalizationReceipt struct {
	Version               string                      `json:"version"`
	Accepted              bool                        `json:"accepted"`
	OutputProduced        bool                        `json:"outputProduced"`
	EffectiveSourceSHA256 string                      `json:"effectiveSourceSHA256,omitempty"`
	BeforeSHA256          string                      `json:"beforeSHA256"`
	AfterSHA256           string                      `json:"afterSHA256"`
	SourceSHA256          string                      `json:"sourceSHA256,omitempty"`
	OriginalSource        string                      `json:"originalSource,omitempty"`
	Outcomes              []ChartNormalizationOutcome `json:"outcomes"`
}
type ChartNormalizationOutcome struct {
	NodeID          string               `json:"nodeId,omitempty"`
	Position        diagnostics.Position `json:"position"`
	Status          string               `json:"status"` // converted or rejected; normalization only, not build success
	Reason          string               `json:"reason,omitempty"`
	OriginalRawJSON string               `json:"originalRawJSON"`
	OriginalSHA256  string               `json:"originalSHA256"`
	Mapping         []ChartCellMapping   `json:"mapping,omitempty"`
}
type ChartCellMapping struct {
	Source string `json:"source"` // JSON pointer in original raw object
	Target string `json:"target"` // JSON pointer in the effective chart
}

func chartHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// NormalizeRectangularCharts is explicitly opt-in. Each chart is validated in full
// before mutation; a rejected chart remains opaque and unchanged for the consumer's gate.
// Only bar, one series and 2..24 complete category/value rows are recognized.
func NormalizeRectangularCharts(doc *ast.AST) (*ast.AST, ChartNormalizationReceipt, error) {
	r := ChartNormalizationReceipt{Version: "rectangular-chart-normalization-v1", Outcomes: []ChartNormalizationOutcome{}}
	if doc == nil {
		return nil, r, fmt.Errorf("nil AST")
	}
	before, err := json.Marshal(doc)
	if err != nil {
		return nil, r, err
	}
	r.BeforeSHA256 = chartHash(before)
	doc = cloneChartValue(reflect.ValueOf(doc)).Interface().(*ast.AST)
	err = ast.Walk(doc, func(n ast.Node) error {
		c, ok := n.(*ast.ChartElement)
		if !ok || (!c.IsJSONMode && len(c.RawJSON) == 0) {
			return nil
		}
		raw := append([]byte(nil), c.RawJSON...)
		o := ChartNormalizationOutcome{NodeID: c.NodeID, Position: c.Position, OriginalRawJSON: string(raw), OriginalSHA256: chartHash(raw), Status: "rejected"}
		data, series, unit, show, mapping, err := decodeRectangularChart(c)
		if err != nil {
			o.Reason = err.Error()
			r.Outcomes = append(r.Outcomes, o)
			return nil
		}
		c.Data, c.Series, c.Unit, c.ShowDataLabels = data, series, unit, show
		c.RawJSON = nil
		c.IsJSONMode = false
		o.Status = "converted"
		o.Mapping = mapping
		r.Outcomes = append(r.Outcomes, o)
		return nil
	})
	if err != nil {
		return nil, r, err
	}
	ast.SetTableContract(doc)
	after, err := json.Marshal(doc)
	if err != nil {
		return nil, r, err
	}
	r.AfterSHA256 = chartHash(after)
	r.Accepted = !r.HasRejections()
	return doc, r, nil
}

func decodeRectangularChart(c *ast.ChartElement) ([][]interface{}, []string, *string, *bool, []ChartCellMapping, error) {
	reject := func(reason string) ([][]interface{}, []string, *string, *bool, []ChartCellMapping, error) {
		return nil, nil, nil, nil, nil, fmt.Errorf("%s", reason)
	}
	if c.ChartType != "bar" || !c.IsJSONMode || len(c.RawJSON) == 0 {
		return reject("requires opaque bar JSON")
	}
	if len(c.Data) != 0 || len(c.Series) != 0 || len(c.Labels) != 0 || len(c.Options) != 0 || len(c.SeriesTypes) != 0 || len(c.SeriesAxes) != 0 || c.Title != "" || c.Unit != nil || c.ShowDataLabels != nil {
		return reject("conflicting canonical chart fields")
	}
	d := json.NewDecoder(bytes.NewReader(c.RawJSON))
	d.UseNumber()
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return reject("requires JSON object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		tok, err = d.Token()
		if err != nil {
			return reject("invalid JSON key")
		}
		key, ok := tok.(string)
		if !ok {
			return reject("invalid JSON key")
		}
		if _, seen := fields[key]; seen {
			return reject("duplicate key: " + key)
		}
		switch key {
		case "data", "series", "unit", "showDataLabels":
		default:
			return reject("unknown key: " + key)
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return reject("invalid JSON value")
		}
		fields[key] = value
	}
	if _, err = d.Token(); err != nil {
		return reject("invalid JSON object")
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return reject("trailing JSON content")
	}
	var rawSeries []interface{}
	if err = json.Unmarshal(fields["series"], &rawSeries); err != nil || len(rawSeries) != 1 {
		return reject("requires exactly one string series")
	}
	seriesName, validSeries := rawSeries[0].(string)
	if !validSeries {
		return reject("requires exactly one string series")
	}
	if !portableChartText(seriesName) {
		return reject("series text cannot round-trip through the native chart grammar")
	}
	series := []string{seriesName}
	var rows [][]interface{}
	rd := json.NewDecoder(bytes.NewReader(fields["data"]))
	rd.UseNumber()
	if err = rd.Decode(&rows); err != nil || len(rows) < 2 || len(rows) > 24 {
		return reject("requires 2..24 rectangular rows")
	}
	mapping := []ChartCellMapping{{Source: "/series/0", Target: "/series/0"}}
	for i, row := range rows {
		if len(row) != 2 {
			return reject("requires category/value rows")
		}
		category, ok := row[0].(string)
		if !ok {
			return reject("category must be a string")
		}
		if !portableChartText(category) {
			return reject("category text cannot round-trip through the native chart grammar")
		}
		number, ok := row[1].(json.Number)
		if !ok {
			return reject("value must be a finite JSON number")
		}
		literal := number.String()
		if len(literal) > 128 {
			return reject("numeric literal exceeds the supported 128-byte profile")
		}
		if e := strings.LastIndexAny(literal, "eE"); e >= 0 {
			exponent, err := strconv.Atoi(literal[e+1:])
			if err != nil || exponent < -400 || exponent > 400 {
				return reject("numeric exponent exceeds the supported -400..400 profile")
			}
		}

		f, e := number.Float64()
		if e != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return reject("value must be a finite JSON number")
		}
		// Consumer ingress uses float64. Reject values whose decimal content
		// would change there, rather than claim preservation in the receipt.
		canonical, e := json.Marshal(f)
		if e != nil {
			return reject("value cannot survive consumer JSON encoding")
		}
		originalValue, originalOK := new(big.Rat).SetString(number.String())
		consumerValue, consumerOK := new(big.Rat).SetString(string(canonical))
		if !originalOK || !consumerOK || originalValue.Cmp(consumerValue) != 0 {
			return reject("value changes under consumer float64 JSON conversion")
		}

		for j := range row {
			pointer := fmt.Sprintf("/data/%d/%d", i, j)
			mapping = append(mapping, ChartCellMapping{Source: pointer, Target: pointer})
		}
	}
	var unit *string
	var show *bool
	if value, has := fields["unit"]; has {
		var u string
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &u) != nil {
			return reject("unit must be a literal string")
		}
		unit = &u
		mapping = append(mapping, ChartCellMapping{Source: "/unit", Target: "/unit"})
	}
	if value, has := fields["showDataLabels"]; has {
		var b bool
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &b) != nil {
			return reject("showDataLabels must be a boolean")
		}
		show = &b
		mapping = append(mapping, ChartCellMapping{Source: "/showDataLabels", Target: "/showDataLabels"})
	}
	return rows, series, unit, show, mapping, nil
}

// HasRejections allows CLI callers to fail closed before generating any output.
func (r ChartNormalizationReceipt) HasRejections() bool {
	for _, o := range r.Outcomes {
		if o.Status == "rejected" {
			return true
		}
	}
	return false
}

// WriteChartNormalizationReceipt publishes an audit sidecar atomically. Source is
// the original input, before include expansion or normalization.
func WriteChartNormalizationReceipt(path string, source []byte, receipt ChartNormalizationReceipt) error {
	receipt.SourceSHA256 = chartHash(source)
	receipt.OriginalSource = string(source)
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".chart-receipt-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Link(name, path)
}

// PublishNormalizedChartSource produces the portable source and its sidecar as
// one operation. Fresh paths are required; existing artifacts are never truncated.
func PublishNormalizedChartSource(output, receiptPath string, original, effective []byte, receipt ChartNormalizationReceipt) error {
	if output == "" || receiptPath == "" {
		return fmt.Errorf("normalization requires --output and --chart-normalization-receipt")
	}
	outAbs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	receiptAbs, err := filepath.Abs(receiptPath)
	if err != nil {
		return err
	}
	if outAbs == receiptAbs {
		return fmt.Errorf("source and receipt paths must differ")
	}
	for _, path := range []string{output, receiptPath} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("normalization output already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	receipt.SourceSHA256 = chartHash(original)
	receipt.OriginalSource = string(original)
	if !receipt.Accepted {
		return WriteChartNormalizationReceipt(receiptPath, original, receipt)
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".chart-source-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err = f.Write(effective); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	receipt.EffectiveSourceSHA256 = chartHash(effective)
	receipt.OutputProduced = true
	// Link rather than rename: publication cannot replace a concurrently created file.
	side, err := os.CreateTemp(filepath.Dir(receiptPath), ".chart-receipt-*")
	if err != nil {
		return err
	}
	sideName := side.Name()
	defer func() { _ = os.Remove(sideName) }()
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		_ = side.Close()
		return err
	}
	b = append(b, '\n')
	if _, err = side.Write(b); err != nil {
		_ = side.Close()
		return err
	}
	if err = side.Close(); err != nil {
		return err
	}
	if err = os.Link(sideName, receiptPath); err != nil {
		return err
	}
	if err = os.Link(name, output); err != nil {
		os.Remove(receiptPath)
		return err
	}
	return nil
}

// cloneChartValue preserves Go numeric values and every source/projection field;
// JSON decoding is deliberately avoided because it coerces authored numbers.
func cloneChartValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(cloneChartValue(v.Elem()))
		return out
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(cloneChartValue(v.Elem()))
		return out
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(cloneChartValue(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), cloneChartValue(iter.Value()))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if out.Field(i).CanSet() && v.Type().Field(i).IsExported() {
				out.Field(i).Set(cloneChartValue(v.Field(i)))
			}
		}
		return out
	default:
		return v
	}
}

// Native historical chart strings do not use JSON escapes. The opt-in profile
// must reject unrepresentable content rather than change that grammar.
func portableChartText(text string) bool {
	for _, r := range text {
		if r == '"' || r == '\\' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
