package unilog

import (
	"fmt"
	"maps"
	"strings"
	"sync"
)

// KeyValueMap is a thread-safe map for log fields optimized for structured logging.
//
// Uses RWMutex for optimal read performance since logging typically involves
// many more reads (formatting, serialization) than writes (setting fields).
//
// It utilizes granular per-field caching to avoid recalculating
// unchanged fields when only some fields are modified between serializations.
type KeyValueMap struct {
	mu       sync.RWMutex
	kv       map[string]any
	prefix   string
	keySep   string
	fieldSep string
	stringer func(k string, v any) string

	// Granular caching: track each field independently
	prefixWithSep    string            // cached prefix + separator
	fieldCache       map[string]string // key -> formatted string cache
	fieldGeneration  map[string]uint64 // key -> generation number
	globalGeneration uint64            // incremented on structural changes

	// Full string caching for when no fields changed
	lastStringResult     string
	lastStringGeneration uint64
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

	return &KeyValueMap{
		kv:              make(map[string]any),
		keySep:          keySegmentSeparator,
		fieldSep:        fieldSeparator,
		stringer:        stringer,
		fieldCache:      make(map[string]string),
		fieldGeneration: make(map[string]uint64),
	}
}

// Get retrieves the value associated with the given raw key and a boolean indicating
// whether the key was found in the map. Prefix is NOT applied to lookups.
func (m *KeyValueMap) Get(key string) (any, bool) {
	m.mu.RLock()
	v, ok := m.kv[key]
	m.mu.RUnlock()
	return v, ok
}

// GetPrefixed retrieves the value associated with the given key,
// applying the current prefix. It also returns a boolean indicating whether the key
// was found in the map.
//
// This is useful when you want to query by the rendered key as seen in logs.
func (m *KeyValueMap) GetPrefixed(key string) (any, bool) {
	m.mu.RLock()
	prefixWithSep := m.prefixWithSep
	m.mu.RUnlock()

	if prefixWithSep == "" {
		return m.Get(key)
	}

	prefixLen := len(prefixWithSep)
	if len(key) > prefixLen && key[:prefixLen] == prefixWithSep {
		return m.Get(key[prefixLen:])
	}

	return nil, false
}

// Set sets key to value with granular cache invalidation.
func (m *KeyValueMap) Set(key string, value any) {
	m.mu.Lock()

	// Check if this is actually a change to avoid unnecessary invalidation
	if oldVal, exists := m.kv[key]; exists && oldVal == value {
		m.mu.Unlock()
		return
	}

	m.kv[key] = value
	m.globalGeneration++

	// Invalidate only this field's cache
	delete(m.fieldCache, key)
	delete(m.fieldGeneration, key)

	// Invalidate full string cache
	m.lastStringResult = ""

	m.mu.Unlock()
}

// SetMultiple sets multiple key-value pairs efficiently with minimal cache invalidation.
// This is more efficient than multiple Set() calls when setting many fields at once.
func (m *KeyValueMap) SetMultiple(pairs map[string]any) {
	if len(pairs) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	for key, value := range pairs {
		// Check if this is actually a change
		if oldVal, exists := m.kv[key]; !exists || oldVal != value {
			m.kv[key] = value
			// Invalidate only this field's cache
			delete(m.fieldCache, key)
			delete(m.fieldGeneration, key)
			changed = true
		}
	}

	if changed {
		m.globalGeneration++
		m.lastStringResult = ""
	}
}

// Delete removes a raw key if present. Prefix is NOT applied.
func (m *KeyValueMap) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.kv[key]; exists {
		delete(m.kv, key)
		delete(m.fieldCache, key)
		delete(m.fieldGeneration, key)
		m.globalGeneration++
		m.lastStringResult = ""
	}
}

// DeletePrefixed removes a key using the current prefix, if present.
func (m *KeyValueMap) DeletePrefixed(key string) {
	m.mu.RLock()
	prefixWithSep := m.prefixWithSep
	m.mu.RUnlock()

	if prefixWithSep == "" {
		m.Delete(key)
		return
	}

	prefixLen := len(prefixWithSep)
	if len(key) > prefixLen && key[:prefixLen] == prefixWithSep {
		m.Delete(key[prefixLen:])
	}
}

// WithPairs returns a new map with the given alternating key-value pairs merged.
// Existing keys are overwritten. Incomplete pairs are skipped.
//
// This creates a new immutable instance.
func (m *KeyValueMap) WithPairs(keyValues ...any) *KeyValueMap {
	if len(keyValues) < 2 {
		return m
	}

	m.mu.RLock()
	newSize := len(m.kv) + len(keyValues)/2
	kvNew := make(map[string]any, newSize)
	maps.Copy(kvNew, m.kv)

	// Copy configuration
	newMap := &KeyValueMap{
		kv:              kvNew,
		prefix:          m.prefix,
		prefixWithSep:   m.prefixWithSep,
		keySep:          m.keySep,
		fieldSep:        m.fieldSep,
		stringer:        m.stringer,
		fieldCache:      make(map[string]string, newSize),
		fieldGeneration: make(map[string]uint64, newSize),
	}

	// Copy existing cache entries that are still valid
	for k, cachedVal := range m.fieldCache {
		if _, stillExists := kvNew[k]; stillExists {
			newMap.fieldCache[k] = cachedVal
			newMap.fieldGeneration[k] = m.fieldGeneration[k]
		}
	}
	m.mu.RUnlock()

	// Add new pairs to the new map
	for i := 0; i < len(keyValues)-1; i += 2 {
		key := toString(keyValues[i])
		newMap.kv[key] = keyValues[i+1]
		// New fields don't have cache entries yet
	}

	newMap.globalGeneration = 1 // Fresh instance

	return newMap
}

// ReplaceAll replaces the entire map with the provided alternating key-value pairs.
// Incomplete pairs are skipped.
func (m *KeyValueMap) ReplaceAll(keyValues ...any) {
	newSize := len(keyValues) / 2
	kvNew := make(map[string]any, newSize)

	if len(keyValues) >= 2 {
		for i := 0; i < len(keyValues)-1; i += 2 {
			kvNew[toString(keyValues[i])] = keyValues[i+1]
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.kv = kvNew
	m.fieldCache = make(map[string]string, newSize)
	m.fieldGeneration = make(map[string]uint64, newSize)
	m.globalGeneration++
	m.lastStringResult = ""
}

// Clear removes all key-value pairs.
func (m *KeyValueMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.kv) > 0 {
		m.kv = make(map[string]any)
		m.fieldCache = make(map[string]string)
		m.fieldGeneration = make(map[string]uint64)
		m.globalGeneration++
		m.lastStringResult = ""
	}
}

// WithPrefix returns a new KeyValueMap with the given prefix added.
// This creates a snapshot copy of the data.
func (m *KeyValueMap) WithPrefix(prefix string) *KeyValueMap {
	if prefix == "" {
		return m
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	kvCopy := make(map[string]any, len(m.kv))
	maps.Copy(kvCopy, m.kv)

	newPrefix := joinPrefix(m.prefix, prefix, m.keySep)
	newPrefixWithSep := newPrefix + m.keySep

	newMap := &KeyValueMap{
		kv:               kvCopy,
		prefix:           newPrefix,
		prefixWithSep:    newPrefixWithSep,
		keySep:           m.keySep,
		fieldSep:         m.fieldSep,
		stringer:         m.stringer,
		fieldCache:       make(map[string]string, len(m.kv)),
		fieldGeneration:  make(map[string]uint64, len(m.kv)),
		globalGeneration: 1,
	}

	// Pre-populate cache with new prefix for existing fields
	for k, v := range newMap.kv {
		newMap.fieldCache[k] = newMap.stringer(newPrefixWithSep+k, v)
		newMap.fieldGeneration[k] = 1
	}

	return newMap
}

// Merge returns a new KeyValueMap that contains all pairs from m and other.
// For duplicate keys, values from other take precedence.
// All other properties of the receiver are preserved.
func (m *KeyValueMap) Merge(other *KeyValueMap) *KeyValueMap {
	m.mu.RLock()
	defer m.mu.RUnlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	kvMerged := make(map[string]any, len(m.kv)+len(other.kv))
	maps.Copy(kvMerged, m.kv)
	maps.Copy(kvMerged, other.kv)

	newMap := &KeyValueMap{
		kv:               kvMerged,
		prefix:           m.prefix,
		prefixWithSep:    m.prefixWithSep,
		keySep:           m.keySep,
		fieldSep:         m.fieldSep,
		stringer:         m.stringer,
		fieldCache:       make(map[string]string, len(kvMerged)),
		fieldGeneration:  make(map[string]uint64, len(kvMerged)),
		globalGeneration: 1,
	}

	// Copy cache entries from both maps, prioritizing 'other' for conflicts
	for k, cachedVal := range m.fieldCache {
		if _, exists := kvMerged[k]; exists {
			newMap.fieldCache[k] = cachedVal
			newMap.fieldGeneration[k] = 1
		}
	}
	for k, cachedVal := range other.fieldCache {
		if _, exists := kvMerged[k]; exists {
			// Values from 'other' take precedence
			if other.prefixWithSep == newMap.prefixWithSep {
				newMap.fieldCache[k] = cachedVal
			} else {
				// Need to recalculate with new prefix
				newMap.fieldCache[k] = newMap.stringer(newMap.prefixWithSep+k, kvMerged[k])
			}
			newMap.fieldGeneration[k] = 1
		}
	}

	return newMap
}

// Clone returns a deep copy of the current map.
func (m *KeyValueMap) Clone() *KeyValueMap {
	m.mu.RLock()
	defer m.mu.RUnlock()

	kvNew := make(map[string]any, len(m.kv))
	fieldCacheNew := make(map[string]string, len(m.fieldCache))
	fieldGenNew := make(map[string]uint64, len(m.fieldGeneration))

	maps.Copy(kvNew, m.kv)
	maps.Copy(fieldCacheNew, m.fieldCache)
	maps.Copy(fieldGenNew, m.fieldGeneration)

	return &KeyValueMap{
		kv:                   kvNew,
		keySep:               m.keySep,
		fieldSep:             m.fieldSep,
		stringer:             m.stringer,
		prefix:               m.prefix,
		prefixWithSep:        m.prefixWithSep,
		fieldCache:           fieldCacheNew,
		fieldGeneration:      fieldGenNew,
		globalGeneration:     m.globalGeneration,
		lastStringResult:     m.lastStringResult,
		lastStringGeneration: m.lastStringGeneration,
	}
}

// AsMap returns the current snapshot of the map, applying the prefix to keys.
//
// IMPORTANT: If no prefix is set, this may return the internal map directly
// for performance. The returned map MUST NOT be modified by the caller.
// If you need to modify the result, use ToMapCopy() instead.
func (m *KeyValueMap) AsMap() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.prefixWithSep == "" {
		// PERFORMANCE OPTIMIZATION: Return internal map directly
		// Caller MUST NOT modify this map
		return m.kv
	}

	// Must create new map when prefix is applied
	out := make(map[string]any, len(m.kv))
	for k, v := range m.kv {
		out[m.prefixWithSep+k] = v
	}

	return out
}

// ToMapCopy returns a new map containing the current snapshot with prefix applied.
// The returned map is always a fresh allocation and may be modified by the caller.
func (m *KeyValueMap) ToMapCopy() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]any, len(m.kv))

	if m.prefixWithSep == "" {
		maps.Copy(out, m.kv)
		return out
	}

	for k, v := range m.kv {
		out[m.prefixWithSep+k] = v
	}

	return out
}

// ToSliceCopy returns a new slice containing the current snapshot of the map
// as a list of key-value pairs, applying the prefix to keys.
// The returned slice is always a fresh allocation and may be modified by the caller.
func (m *KeyValueMap) ToSliceCopy() []any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]any, 0, len(m.kv)*2)

	if m.prefixWithSep == "" {
		for k, v := range m.kv {
			out = append(out, k, v)
		}
	} else {
		for k, v := range m.kv {
			out = append(out, m.prefixWithSep+k, v)
		}
	}

	return out
}

// Len returns the number of key-value pairs.
func (m *KeyValueMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.kv)
}

// Range calls fn for each key/value pair of the current snapshot.
func (m *KeyValueMap) Range(fn func(k string, v any)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.prefixWithSep == "" {
		for k, v := range m.kv {
			fn(k, v)
		}
	} else {
		for k, v := range m.kv {
			fn(m.prefixWithSep+k, v)
		}
	}
}

// String returns a string representation with granular caching optimization.
//
// This is heavily optimized for logging. Only recalculates formatting for fields that
// have changed since the last call, preserving cached results for unchanged fields.
func (m *KeyValueMap) String() string {
	m.mu.RLock()

	if len(m.kv) == 0 {
		m.mu.RUnlock()
		return ""
	}

	// Fast path: check if full string is still valid
	if m.lastStringResult != "" && m.lastStringGeneration == m.globalGeneration {
		result := m.lastStringResult
		m.mu.RUnlock()
		return result
	}

	// Need to rebuild, but use granular caching
	m.mu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.lastStringResult != "" && m.lastStringGeneration == m.globalGeneration {
		return m.lastStringResult
	}

	// Build formatted strings, using cache where possible
	formattedParts := make([]string, 0, len(m.kv))

	for k, v := range m.kv {
		var formatted string

		// Check if we have a valid cached entry for this field
		if cachedFormatted, exists := m.fieldCache[k]; exists &&
			m.fieldGeneration[k] == m.globalGeneration {
			// Use cached value
			formatted = cachedFormatted
		} else {
			// Calculate and cache new value
			if m.prefixWithSep == "" {
				formatted = m.stringer(k, v)
			} else {
				formatted = m.stringer(m.prefixWithSep+k, v)
			}
			m.fieldCache[k] = formatted
			m.fieldGeneration[k] = m.globalGeneration
		}

		formattedParts = append(formattedParts, formatted)
	}

	// Build final string
	result := strings.Join(formattedParts, m.fieldSep)

	// Cache the full result
	m.lastStringResult = result
	m.lastStringGeneration = m.globalGeneration

	return result
}

// toString is a helper function that returns the string representation of a value.
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return itoa64(int64(val))
	case int64:
		return itoa64(val)
	case uint:
		return utoa64(uint64(val))
	case uint64:
		return utoa64(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func itoa64(val int64) string {
	if val == 0 {
		return "0"
	}

	var buf [20]byte // max int64 = 19 digits + optional sign
	i := len(buf)

	negative := val < 0
	if negative {
		val = -val
	}

	for val > 0 {
		i--
		buf[i] = byte('0' + val%10)
		val /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

func utoa64(val uint64) string {
	if val == 0 {
		return "0"
	}

	var buf [20]byte // max uint64 = 20 digits
	i := len(buf)

	for val > 0 {
		i--
		buf[i] = byte('0' + val%10)
		val /= 10
	}

	return string(buf[i:])
}

// joinPrefix combines two prefixes with a separator.
func joinPrefix(existing string, new string, separator string) string {
	if existing == "" {
		return new
	}
	if new == "" {
		return existing
	}

	// Pre-allocate exact size needed
	buf := make([]byte, 0, len(existing)+len(separator)+len(new))
	buf = append(buf, existing...)
	buf = append(buf, separator...)
	buf = append(buf, new...)

	return string(buf)
}
