package unilog

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestNewKeyValueMap(t *testing.T) {
	// Comparing function pointers in `want` is not reliable.
	// We will test the behavior of the stringer instead of deep equality of the struct.
	t.Run("with default stringer", func(t *testing.T) {
		got := NewKeyValueMap(".", " ", nil)
		if got.keySep != "." || got.fieldSep != " " {
			t.Errorf("NewKeyValueMap() separators incorrect, got keySep=%s, fieldSep=%s", got.keySep, got.fieldSep)
		}
		if got.stringer == nil {
			t.Fatal("NewKeyValueMap() with nil stringer resulted in a nil stringer field")
		}
		// Test the behavior of the default stringer
		if s := got.stringer("key", "val"); s != "key=val" {
			t.Errorf("Default stringer produced wrong output: got %s, want key=val", s)
		}
	})

	t.Run("with custom stringer", func(t *testing.T) {
		customStringer := func(k string, v any) string { return fmt.Sprintf("%s->%v", k, v) }
		got := NewKeyValueMap(":", ";", customStringer)

		if got.stringer == nil {
			t.Fatal("NewKeyValueMap() with custom stringer resulted in a nil stringer field")
		}
		// Test the behavior of the custom stringer
		if s := got.stringer("key", "val"); s != "key->val" {
			t.Errorf("Custom stringer produced wrong output: got %s, want key->val", s)
		}
	})
}

func TestKeyValueMap_Get(t *testing.T) {
	// Helper to create a map with test data
	createTestMap := func(data map[string]any) *KeyValueMap {
		m := NewKeyValueMap(".", " ", nil)
		for k, v := range data {
			m.Set(k, v)
		}
		return m
	}

	type args struct {
		key string
	}
	tests := []struct {
		name     string
		testData map[string]any
		args     args
		want     any
		want1    bool
	}{
		{
			name:     "get existing key",
			testData: map[string]any{"a": 1, "b": "hello"},
			args:     args{key: "a"},
			want:     1,
			want1:    true,
		},
		{
			name:     "get non-existent key",
			testData: map[string]any{"a": 1, "b": "hello"},
			args:     args{key: "c"},
			want:     nil,
			want1:    false,
		},
		{
			name:     "get from empty map",
			testData: nil,
			args:     args{key: "a"},
			want:     nil,
			want1:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestMap(tt.testData)
			got, got1 := m.Get(tt.args.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KeyValueMap.Get() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("KeyValueMap.Get() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestKeyValueMap_GetPrefixed(t *testing.T) {
	// Helper to create a map with test data and configuration
	createTestMap := func(data map[string]any, prefix, keySep string) *KeyValueMap {
		m := NewKeyValueMap(keySep, " ", nil)
		for k, v := range data {
			m.Set(k, v)
		}
		return m.WithPrefix(prefix)
	}

	type args struct {
		key string
	}
	tests := []struct {
		name     string
		testData map[string]any
		prefix   string
		keySep   string
		args     args
		want     any
		want1    bool
	}{
		{
			name:     "get with correct prefix",
			testData: map[string]any{"key1": 100},
			prefix:   "p1",
			keySep:   ".",
			args:     args{key: "p1.key1"},
			want:     100,
			want1:    true,
		},
		{
			name:     "get with incorrect prefix",
			testData: map[string]any{"key1": 100},
			prefix:   "p1",
			keySep:   ".",
			args:     args{key: "p2.key1"},
			want:     nil,
			want1:    false,
		},
		{
			name:     "get non-existent key with correct prefix",
			testData: map[string]any{"key1": 100},
			prefix:   "p1",
			keySep:   ".",
			args:     args{key: "p1.key2"},
			want:     nil,
			want1:    false,
		},
		{
			name:     "get without prefix on a prefixed map",
			testData: map[string]any{"key1": 100},
			prefix:   "p1",
			keySep:   ".",
			args:     args{key: "key1"},
			want:     nil,
			want1:    false,
		},
		{
			name:     "get on a map with no prefix",
			testData: map[string]any{"key1": 100},
			prefix:   "",
			keySep:   ".",
			args:     args{key: "key1"},
			want:     100,
			want1:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestMap(tt.testData, tt.prefix, tt.keySep)
			got, got1 := m.GetPrefixed(tt.args.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KeyValueMap.GetPrefixed() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("KeyValueMap.GetPrefixed() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestKeyValueMap_Set(t *testing.T) {
	// This test needs to verify the state of the map after the operation.
	t.Run("set new key", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		val, ok := m.Get("a")
		if !ok || val != 1 {
			t.Errorf("Set failed, expected to find key 'a' with value 1, got val=%v, ok=%t", val, ok)
		}
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.Set("a", 2)
		val, ok := m.Get("a")
		if !ok || val != 2 {
			t.Errorf("Set overwrite failed, expected to find key 'a' with value 2, got val=%v, ok=%t", val, ok)
		}
	})
}

func TestKeyValueMap_Delete(t *testing.T) {
	// This test needs to verify the state of the map after the operation.
	t.Run("delete existing key", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.Set("b", 2)
		m.Delete("a")
		if _, ok := m.Get("a"); ok {
			t.Error("Delete failed, key 'a' should not exist")
		}
		if m.Len() != 1 {
			t.Errorf("Delete failed, expected map length 1, got %d", m.Len())
		}
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.Delete("b")
		if m.Len() != 1 {
			t.Errorf("Delete non-existent key should not change map length, got %d", m.Len())
		}
	})
}

func TestKeyValueMap_DeletePrefixed(t *testing.T) {
	// This test needs to verify the state of the map after the operation.
	t.Run("delete with correct prefix", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil).WithPrefix("p1")
		m.Set("key1", 1)
		m.DeletePrefixed("p1.key1")
		if m.Len() != 0 {
			t.Errorf("DeletePrefixed failed, expected map length 0, got %d", m.Len())
		}
	})

	t.Run("delete with incorrect prefix", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil).WithPrefix("p1")
		m.Set("key1", 1)
		m.DeletePrefixed("p2.key1")
		if m.Len() != 1 {
			t.Errorf("DeletePrefixed with wrong prefix should not change map length, got %d", m.Len())
		}
	})

	t.Run("delete on map with no prefix", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("key1", 1)
		m.DeletePrefixed("key1")
		if m.Len() != 0 {
			t.Errorf("DeletePrefixed on non-prefixed map failed, expected length 0, got %d", m.Len())
		}
	})
}

func TestKeyValueMap_WithPairs(t *testing.T) {
	baseMap := NewKeyValueMap(".", " ", nil)
	baseMap.Set("a", 1)

	type args struct {
		keyValues []any
	}
	tests := []struct {
		name string
		args args
		want map[string]any
	}{
		{
			name: "add new pairs",
			args: args{keyValues: []any{"b", 2, "c", 3}},
			want: map[string]any{"a": 1, "b": 2, "c": 3},
		},
		{
			name: "incomplete pair",
			args: args{keyValues: []any{"b", 2, "c"}},
			want: map[string]any{"a": 1, "b": 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh map for each test run to avoid mutation issues
			m := NewKeyValueMap(".", " ", nil)
			m.Set("a", 1)

			got := m.WithPairs(tt.args.keyValues...)

			// Check the resulting map data
			gotData := got.ToMapCopy()
			if !reflect.DeepEqual(gotData, tt.want) {
				t.Errorf("KeyValueMap.WithPairs() = %v, want %v", gotData, tt.want)
			}
			// Ensure original map is not modified
			if m.Len() != 1 {
				t.Error("Original map was modified by WithPairs()")
			}
		})
	}

	t.Run("no pairs", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)

		got := m.WithPairs()

		// Should return the same instance for empty keyValues
		if got != m {
			t.Errorf("WithPairs() with no pairs should return the same instance")
		}
	})
}

func TestKeyValueMap_ReplaceAll(t *testing.T) {
	t.Run("replace existing map", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.ReplaceAll("b", 2, "c", 3)
		expected := map[string]any{"b": 2, "c": 3}
		actual := m.ToMapCopy()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("ReplaceAll() got %v, want %v", actual, expected)
		}
	})

	t.Run("replace with empty slice", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.ReplaceAll()
		if m.Len() != 0 {
			t.Errorf("ReplaceAll() with empty slice should result in an empty map, got len %d", m.Len())
		}
	})
}

func TestKeyValueMap_Clear(t *testing.T) {
	t.Run("clear a non-empty map", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.Clear()
		if m.Len() != 0 {
			t.Errorf("Clear() should result in an empty map, got len %d", m.Len())
		}
	})
}

func TestKeyValueMap_WithPrefix(t *testing.T) {
	baseMap := NewKeyValueMap(".", " ", nil)
	baseMap.Set("key", "val")

	t.Run("add a prefix", func(t *testing.T) {
		got := baseMap.WithPrefix("p1")
		if got.prefix != "p1" {
			t.Errorf("WithPrefix() prefix = %v, want %v", got.prefix, "p1")
		}
		// With RWMutex implementation, we cannot share the underlying map
		// because we cannot share mutexes safely. This behavior change is expected.
		// Verify the data is correctly copied instead
		if val, ok := got.Get("key"); !ok || val != "val" {
			t.Errorf("WithPrefix() should preserve data, got val=%v, ok=%t", val, ok)
		}
	})

	t.Run("add a nested prefix", func(t *testing.T) {
		got := baseMap.WithPrefix("p1").WithPrefix("p2")
		if got.prefix != "p1.p2" {
			t.Errorf("WithPrefix() nested prefix = %v, want %v", got.prefix, "p1.p2")
		}
	})

	t.Run("empty prefix returns same instance", func(t *testing.T) {
		got := baseMap.WithPrefix("")
		if got != baseMap {
			t.Error("WithPrefix(\"\") should return the same instance")
		}
	})
}

func TestKeyValueMap_Merge(t *testing.T) {
	mapA := NewKeyValueMap(".", " ", nil)
	mapA.Set("a", 1)
	mapA.Set("b", 2)

	mapB := NewKeyValueMap(".", " ", nil)
	mapB.Set("b", 99)
	mapB.Set("c", 3)

	t.Run("merge two maps", func(t *testing.T) {
		got := mapA.Merge(mapB)
		expected := map[string]any{"a": 1, "b": 99, "c": 3}
		actual := got.ToMapCopy()
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Merge() = %v, want %v", actual, expected)
		}
		// Ensure original maps are not modified
		if mapA.Len() != 2 || mapB.Len() != 2 {
			t.Error("Original maps were modified by Merge()")
		}
	})
}

func TestKeyValueMap_Clone(t *testing.T) {
	t.Run("clone a map", func(t *testing.T) {
		original := NewKeyValueMap(".", " ", nil).WithPrefix("p1")
		original.Set("a", 1)

		clone := original.Clone()

		originalData := original.ToMapCopy()
		cloneData := clone.ToMapCopy()
		if !reflect.DeepEqual(originalData, cloneData) {
			t.Error("Clone() data should be equal")
		}

		if original.prefix != clone.prefix {
			t.Errorf("Clone() prefix mismatch, got %s, want %s", clone.prefix, original.prefix)
		}

		// Modify clone and check original
		clone.Set("b", 2)
		if _, ok := original.Get("b"); ok {
			t.Error("Modifying clone affected the original map")
		}
	})
}

func TestKeyValueMap_AsMap(t *testing.T) {
	tests := []struct {
		name     string
		testData map[string]any
		prefix   string
		keySep   string
		want     map[string]any
	}{
		{
			name:     "map without prefix",
			testData: map[string]any{"a": 1},
			prefix:   "",
			keySep:   ".",
			want:     map[string]any{"a": 1},
		},
		{
			name:     "map with prefix",
			testData: map[string]any{"a": 1},
			prefix:   "p1",
			keySep:   ".",
			want:     map[string]any{"p1.a": 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewKeyValueMap(tt.keySep, " ", nil)
			for k, v := range tt.testData {
				m.Set(k, v)
			}
			if tt.prefix != "" {
				m = m.WithPrefix(tt.prefix)
			}

			if got := m.AsMap(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KeyValueMap.AsMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyValueMap_ToMapCopy(t *testing.T) {
	originalMap := NewKeyValueMap(".", " ", nil)
	originalMap.Set("a", 1)

	t.Run("copy and modify", func(t *testing.T) {
		copiedMap := originalMap.ToMapCopy()
		copiedMap["b"] = 2 // Modify the copy

		if _, ok := originalMap.Get("b"); ok {
			t.Error("Modifying the copy from ToMapCopy() affected the original map")
		}
	})

	t.Run("copy with prefix", func(t *testing.T) {
		prefixedMap := originalMap.WithPrefix("p1")
		copiedMap := prefixedMap.ToMapCopy()

		expected := map[string]any{"p1.a": 1}
		if !reflect.DeepEqual(copiedMap, expected) {
			t.Errorf("ToMapCopy() with prefix = %v, want %v", copiedMap, expected)
		}
	})
}

// sortablePairs helps sort a flat slice of key-value pairs.
type sortablePairs struct {
	slice []any
}

func (p sortablePairs) Len() int { return len(p.slice) / 2 }
func (p sortablePairs) Swap(i, j int) {
	// Calculate the actual slice indices for the pairs
	i_key, j_key := i*2, j*2
	i_val, j_val := i*2+1, j*2+1

	// Swap keys
	p.slice[i_key], p.slice[j_key] = p.slice[j_key], p.slice[i_key]
	// Swap values
	p.slice[i_val], p.slice[j_val] = p.slice[j_val], p.slice[i_val]
}
func (p sortablePairs) Less(i, j int) bool {
	// Compare keys at the calculated indices
	return fmt.Sprint(p.slice[i*2]) < fmt.Sprint(p.slice[j*2])
}

func TestKeyValueMap_ToSliceCopy(t *testing.T) {
	// Since map iteration order is not guaranteed, we sort the slices for stable comparison.
	sorter := func(s []any) {
		if len(s) < 2 {
			return
		}
		sort.Sort(sortablePairs{slice: s})
	}

	tests := []struct {
		name     string
		testData map[string]any
		prefix   string
		keySep   string
		want     []any
	}{
		{
			name:     "slice without prefix",
			testData: map[string]any{"b": 2, "a": 1},
			prefix:   "",
			keySep:   ".",
			want:     []any{"a", 1, "b", 2},
		},
		{
			name:     "slice with prefix",
			testData: map[string]any{"b": 2, "a": 1},
			prefix:   "p",
			keySep:   ".",
			want:     []any{"p.a", 1, "p.b", 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewKeyValueMap(tt.keySep, " ", nil)
			for k, v := range tt.testData {
				m.Set(k, v)
			}
			if tt.prefix != "" {
				m = m.WithPrefix(tt.prefix)
			}

			got := m.ToSliceCopy()

			// Sort both slices to ensure order doesn't affect the test
			sorter(got)
			sorter(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KeyValueMap.ToSliceCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyValueMap_Len(t *testing.T) {
	tests := []struct {
		name     string
		testData map[string]any
		want     int
	}{
		{
			name:     "empty map",
			testData: nil,
			want:     0,
		},
		{
			name:     "map with items",
			testData: map[string]any{"a": 1, "b": 2},
			want:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewKeyValueMap(".", " ", nil)
			for k, v := range tt.testData {
				m.Set(k, v)
			}
			if got := m.Len(); got != tt.want {
				t.Errorf("KeyValueMap.Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyValueMap_Range(t *testing.T) {
	t.Run("range over map with prefix", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil).WithPrefix("p")
		m.Set("a", 1)
		m.Set("b", 2)

		results := make(map[string]any)
		m.Range(func(k string, v any) {
			results[k] = v
		})

		expected := map[string]any{"p.a": 1, "p.b": 2}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("Range() results = %v, want %v", results, expected)
		}
	})

	t.Run("range over map without prefix", func(t *testing.T) {
		m := NewKeyValueMap(".", " ", nil)
		m.Set("a", 1)
		m.Set("b", 2)

		results := make(map[string]any)
		m.Range(func(k string, v any) {
			results[k] = v
		})

		expected := map[string]any{"a": 1, "b": 2}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("Range() results = %v, want %v", results, expected)
		}
	})
}

func TestKeyValueMap_String(t *testing.T) {
	m := NewKeyValueMap(".", " ", nil)
	m.Set("a", 1)
	m.Set("b", 2)

	got := m.String()

	// To validate without depending on order, we split the output
	// and check that all expected parts are present.
	parts := strings.Split(got, " ")
	if len(parts) != 2 {
		t.Fatalf("String() expected 2 parts, got %d from output %q", len(parts), got)
	}

	// Use a map to easily check for the presence of each part.
	gotParts := make(map[string]bool)
	for _, p := range parts {
		gotParts[p] = true
	}

	expectedParts := []string{"a=1", "b=2"}
	for _, p := range expectedParts {
		if !gotParts[p] {
			t.Errorf("String() output %q is missing expected part %q", got, p)
		}
	}
}

func TestKeyValueMap_String_WithPrefix(t *testing.T) {
	m := NewKeyValueMap(".", " ", nil).WithPrefix("pre")
	m.Set("a", 1)
	m.Set("b", 2)

	got := m.String()
	parts := strings.Split(got, " ")
	if len(parts) != 2 {
		t.Fatalf("String() with prefix expected 2 parts, got %d from output %q", len(parts), got)
	}

	gotParts := make(map[string]bool)
	for _, p := range parts {
		gotParts[p] = true
	}

	expectedParts := []string{"pre.a=1", "pre.b=2"}
	for _, p := range expectedParts {
		if !gotParts[p] {
			t.Errorf("String() with prefix output %q is missing expected part %q", got, p)
		}
	}
}

func TestKeyValueMap_String_Empty(t *testing.T) {
	m := NewKeyValueMap(".", " ", nil)
	got := m.String()
	if got != "" {
		t.Errorf("String() on empty map should return empty string, got %q", got)
	}
}

func Test_toString(t *testing.T) {
	type args struct {
		v any
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "string", args: args{v: "hello"}, want: "hello"},
		{name: "int", args: args{v: 123}, want: "123"},
		{name: "bool", args: args{v: true}, want: "true"},
		{name: "nil", args: args{v: nil}, want: "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toString(tt.args.v); got != tt.want {
				t.Errorf("toString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_joinPrefix(t *testing.T) {
	type args struct {
		existing  string
		new       string
		separator string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "both present", args: args{"a", "b", "."}, want: "a.b"},
		{name: "existing empty", args: args{"", "b", "."}, want: "b"},
		{name: "new empty", args: args{"a", "", "."}, want: "a"},
		{name: "both empty", args: args{"", "", "."}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinPrefix(tt.args.existing, tt.args.new, tt.args.separator); got != tt.want {
				t.Errorf("joinPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test for concurrency safety.
func TestKeyValueMap_Concurrency(t *testing.T) {
	m := NewKeyValueMap(".", " ", nil)
	var wg sync.WaitGroup
	numRoutines := 50
	numWrites := 50

	// Concurrent writes
	for i := range numRoutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				key := fmt.Sprintf("key-%d-%d", i, j)
				m.Set(key, i*j)
			}
		}(i)
	}

	// Concurrent reads
	for range numRoutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Range(func(k string, v any) {
				// Read operation
			})
			_ = m.String()
			_ = m.AsMap()
		}()
	}

	wg.Wait()

	expectedLen := numRoutines * numWrites
	if m.Len() != expectedLen {
		t.Errorf("expected final length %d, got %d", expectedLen, m.Len())
	}
}
