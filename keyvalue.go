package unilog

import (
	"fmt"
	"maps"
	"strings"
	"sync/atomic"
)

// KeyValueMap is a thread-safe, copy-on-write map for log fields.
//
// Reads are lock-free; writes create a new immutable snapshot and swap it atomically.
//
// It supports lazy prefixing: a prefix string is stored and applied
// when rendering or iterating, without rewriting the underlying map.
type KeyValueMap struct {
	kv       atomic.Value // holds map[string]any
	prefix   string
	keySep   string
	fieldSep string
	stringer func(k string, v any) string
}

// Ensure KeyValueMap implements the fmt.Stringer interface.
var _ fmt.Stringer = (*KeyValueMap)(nil)

// NewKeyValueMap creates a new KeyValueMap with the given key segment separator and field separator.
// Stringer is a function that returns a string representation of a key-value pair.
// If stringer is nil, the default stringer is used.
func NewKeyValueMap(keySegmentSeparator string, fieldSeparator string, stringer func(k string, v any) string) *KeyValueMap {
	if stringer == nil {
		stringer = func(k string, v any) string {
			return k + "=" + fmt.Sprint(v)
		}
	}

	m := &KeyValueMap{
		keySep:   keySegmentSeparator,
		fieldSep: fieldSeparator,
		stringer: stringer,
	}
	m.kv.Store(make(map[string]any))

	return m
}

// Get retrieves the value associated with the given raw key and a boolean indicating
// whether the key was found in the map. Prefix is NOT applied to lookups.
func (m *KeyValueMap) Get(key string) (any, bool) {
	kv := m.kv.Load().(map[string]any)
	v, ok := kv[key]

	return v, ok
}

// GetPrefixed retrieves the value associated with the given key,
// applying the current prefix. It also returns a boolean indicating whether the key
// was found in the map.
//
// This is useful when you want to query by the rendered key as seen in logs.
func (m *KeyValueMap) GetPrefixed(key string) (any, bool) {
	if m.prefix == "" {
		return m.Get(key)
	}

	if lookup, ok := strings.CutPrefix(key, m.prefix+m.keySep); ok {
		return m.Get(lookup)
	}

	return nil, false
}

// Set sets key to value using copy-on-write semantics.
func (m *KeyValueMap) Set(key string, value any) {
	kvOld := m.load()
	kvNew := make(map[string]any, len(kvOld)+1)
	maps.Copy(kvNew, kvOld)
	kvNew[key] = value
	m.kv.Store(kvNew)
}

// Delete removes a raw key if present using copy-on-write semantics.
// Prefix is NOT applied.
func (m *KeyValueMap) Delete(key string) {
	kvOld := m.load()

	if _, exists := kvOld[key]; !exists {
		return
	}

	kvNew := make(map[string]any, len(kvOld)-1)
	for k, v := range kvOld {
		if k != key {
			kvNew[k] = v
		}
	}

	m.kv.Store(kvNew)
}

// DeletePrefixed removes a key using the current prefix, if present.
func (m *KeyValueMap) DeletePrefixed(key string) {
	if m.prefix == "" {
		m.Delete(key)
		return
	}

	if lookup, ok := strings.CutPrefix(key, m.prefix+m.keySep); ok {
		m.Delete(lookup)
	}
}

// WithPairs returns a new map with the given alternating key-value pairs merged.
// Existing keys are overwritten. Incomplete pairs are skipped.
func (m *KeyValueMap) WithPairs(keyValues ...any) *KeyValueMap {
	if len(keyValues) < 2 {
		return m
	}

	kvOld := m.load()
	kvNew := make(map[string]any, len(kvOld)+len(keyValues)/2)
	maps.Copy(kvNew, kvOld)

	for i := 0; i < len(keyValues)-1; i += 2 {
		kvNew[toString(keyValues[i])] = keyValues[i+1]
	}

	out := &KeyValueMap{
		prefix:   m.prefix,
		keySep:   m.keySep,
		fieldSep: m.fieldSep,
		stringer: m.stringer,
	}
	out.kv.Store(kvNew)

	return out
}

// ReplaceAll replaces the entire map with the provided alternating key-value pairs.
// Incomplete pairs are skipped.
func (m *KeyValueMap) ReplaceAll(keyValues []any) {
	if len(keyValues) < 2 {
		m.kv.Store(make(map[string]any))
		return
	}
	kv := make(map[string]any, len(keyValues)/2)
	for i := 0; i < len(keyValues)-1; i += 2 {
		kv[toString(keyValues[i])] = keyValues[i+1]
	}
	m.kv.Store(kv)
}

// Clear removes all key-value pairs.
func (m *KeyValueMap) Clear() {
	m.kv.Store(make(map[string]any))
}

// WithPrefix returns a new KeyValueMap with the given prefix added lazily.
// The underlying map is shared; prefix is only applied when rendering.
func (m *KeyValueMap) WithPrefix(prefix string) *KeyValueMap {
	if prefix == "" {
		return m
	}

	return &KeyValueMap{
		kv:       m.kv,
		prefix:   joinPrefix(m.prefix, prefix, m.keySep),
		keySep:   m.keySep,
		fieldSep: m.fieldSep,
		stringer: m.stringer,
	}
}

// Merge returns a new KeyValueMap that contains all pairs from m and other.
// For duplicate keys, values from other take precedence.
// All other properties of the receiver is preserved.
func (m *KeyValueMap) Merge(other *KeyValueMap) *KeyValueMap {
	kvA := m.load()
	kvB := other.load()
	kvMerged := make(map[string]any, len(kvA)+len(kvB))
	maps.Copy(kvMerged, kvA)
	maps.Copy(kvMerged, kvB)

	out := &KeyValueMap{
		prefix:   m.prefix,
		keySep:   m.keySep,
		fieldSep: m.fieldSep,
		stringer: m.stringer,
	}
	out.kv.Store(kvMerged)

	return out
}

// Clone returns a deep copy of the current map.
func (m *KeyValueMap) Clone() *KeyValueMap {
	kvOld := m.load()
	kvNew := make(map[string]any, len(kvOld))
	maps.Copy(kvNew, kvOld)

	out := &KeyValueMap{
		keySep:   m.keySep,
		fieldSep: m.fieldSep,
		stringer: m.stringer,
		prefix:   m.prefix,
	}
	out.kv.Store(kvNew)

	return out
}

// AsMap returns the current snapshot of the map, applying the prefix to keys.
// If no prefix is set, ToMap may return the internal map without copying.
// The returned map must not be modified by the caller.
func (m *KeyValueMap) AsMap() map[string]any {
	kv := m.load()
	if m.prefix == "" {
		return kv
	}

	out := make(map[string]any, len(kv))
	prefix := m.prefix + m.keySep
	for k, v := range kv {
		out[prefix+k] = v
	}

	return out
}

// ToMapCopy returns a new map containing the current snapshot with prefix applied.
// The returned map is always a fresh allocation and may be modified by the caller.
func (m *KeyValueMap) ToMapCopy() map[string]any {
	kv := m.load()
	out := make(map[string]any, len(kv))
	if m.prefix == "" {
		// copy to ensure caller can mutate safely
		maps.Copy(out, kv)
		return out
	}

	prefix := m.prefix + m.keySep
	for k, v := range kv {
		out[prefix+k] = v
	}

	return out
}

// ToSliceCopy returns a new slice containing the current snapshot of the map
// as a list of key-value pairs, applying the prefix to keys.
// The returned slice is always a fresh allocation and may be modified by the caller.
func (m *KeyValueMap) ToSliceCopy() []any {
	kv := m.load()
	out := make([]any, 0, len(kv)*2)
	var prefix string
	if m.prefix != "" {
		for k, v := range kv {
			out = append(out, k, v)
		}
	} else {
		prefix = m.prefix + m.keySep
		for k, v := range kv {
			out = append(out, prefix+k, v)
		}
	}
	return out
}

// Len returns the number of key-value pairs.
func (m *KeyValueMap) Len() int {
	return len(m.load())
}

// Range calls fn for each key/value pair of the current snapshot,
// applying the prefix lazily.
func (m *KeyValueMap) Range(fn func(k string, v any)) {
	kv := m.load()
	if m.prefix == "" {
		for k, v := range kv {
			fn(k, v)
		}
	} else {
		prefix := m.prefix + m.keySep
		for k, v := range kv {
			fn(prefix+k, v)
		}
	}
}

// String returns a string representation of the current map,
// applying the prefix lazily.
func (m *KeyValueMap) String() string {
	kv := m.load()
	if len(kv) == 0 {
		return ""
	}

	var sb strings.Builder
	prefix := m.prefix
	if prefix != "" {
		prefix += m.keySep
	}
	first := true
	for k, v := range kv {
		if !first {
			sb.WriteString(m.fieldSep)
		}
		sb.WriteString(m.stringer(prefix+k, v))
		first = false
	}
	return sb.String()
}

// load returns the current snapshot, ensuring a non-nil, type-stable map.
func (m *KeyValueMap) load() map[string]any {
	return m.kv.Load().(map[string]any)
}

// toString is a helper function that returns the string representation of a value.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func joinPrefix(existing string, new string, separator string) string {
	if existing == "" {
		return new
	}
	if new == "" {
		return existing
	}
	return existing + separator + new
}
