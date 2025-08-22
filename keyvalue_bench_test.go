package unilog

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// Number of key-value pairs to generate (none, average, high)
var keySizes = []int{2, 16, 64}

// Benchmark data generation helpers
func generateKeys(n int, prefix string) []string {
	keys := make([]string, n)
	for i := range n {
		keys[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return keys
}

func generateKeyValuePairs(n int, prefix string) []any {
	pairs := make([]any, 0, n*2)
	for i := range n {
		pairs = append(pairs, fmt.Sprintf("%s%d", prefix, i), i)
	}
	return pairs
}

func createPopulatedMap(n int) *KeyValueMap {
	m := NewKeyValueMap(".", " ", nil)
	for i := range n {
		m.Set(fmt.Sprintf("key%d", i), i)
	}
	return m
}

// Basic Operations Benchmarks

func BenchmarkKeyValueMap_Set(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := NewKeyValueMap(".", " ", nil)
			keys := generateKeys(size, "key")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; b.Loop(); i++ {
				idx := i % size
				m.Set(keys[idx], i)
			}
		})
	}
}

func BenchmarkKeyValueMap_Get(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size)
			keys := generateKeys(size, "key")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; b.Loop(); i++ {
				idx := i % size
				_, _ = m.Get(keys[idx])
			}
		})
	}
}

func BenchmarkKeyValueMap_GetPrefixed(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size).WithPrefix("prefix")
			prefixedKeys := generateKeys(size, "prefix.key")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; b.Loop(); i++ {
				idx := i % size
				_, _ = m.GetPrefixed(prefixedKeys[idx])
			}
		})
	}
}

func BenchmarkKeyValueMap_Delete(b *testing.B) {
	b.Run("delete_existing", func(b *testing.B) {
		const poolSize = 1024 // small, fixed pool instead of b.N

		// Build a pool of maps where "key50" exists
		pool := make([]*KeyValueMap, poolSize)
		for i := range poolSize {
			pool[i] = createPopulatedMap(100)
		}

		b.ResetTimer()
		b.ReportAllocs()

		idx := 0

		for b.Loop() {
			pool[idx].Delete("key50")
			idx++
			if idx == poolSize {
				// Restore the pool outside of timing so Set() isn’t counted
				b.StopTimer()
				for j := 0; j < poolSize; j++ {
					pool[j].Set("key50", 50)
				}
				b.StartTimer()
				idx = 0
			}
		}
	})

	b.Run("delete_non_existing", func(b *testing.B) {
		// Only need a single map; deleting a missing key is idempotent
		m := createPopulatedMap(100)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			m.Delete("non_existing_key")
		}
	})
}

// Immutable Operations Benchmarks

func BenchmarkKeyValueMap_WithPairs(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("pairs_%d", size), func(b *testing.B) {
			baseMap := createPopulatedMap(10)
			pairs := generateKeyValuePairs(size, "new_key")

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = baseMap.WithPairs(pairs...)
			}
		})
	}

}

func BenchmarkKeyValueMap_WithPrefix(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = m.WithPrefix("benchmark_prefix")
			}
		})
	}
}

func BenchmarkKeyValueMap_Merge(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m1 := createPopulatedMap(size)
			m2 := createPopulatedMap(size / 2)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = m1.Merge(m2)
			}
		})
	}
}

func BenchmarkKeyValueMap_Clone(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = m.Clone()
			}
		})
	}
}

// Output Operations Benchmarks

func BenchmarkKeyValueMap_AsMap(b *testing.B) {
	b.Run("without_prefix", func(b *testing.B) {
		for _, size := range keySizes {
			b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
				m := createPopulatedMap(size)

				b.ResetTimer()
				b.ReportAllocs()

				for b.Loop() {
					_ = m.AsMap()
				}
			})
		}
	})

	b.Run("with_prefix", func(b *testing.B) {
		for _, size := range keySizes {
			b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
				m := createPopulatedMap(size).WithPrefix("bench")

				b.ResetTimer()
				b.ReportAllocs()

				for b.Loop() {
					_ = m.AsMap()
				}
			})
		}
	})
}

func BenchmarkKeyValueMap_ToMapCopy(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = m.ToMapCopy()
			}
		})
	}
}

func BenchmarkKeyValueMap_ToSliceCopy(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = m.ToSliceCopy()
			}
		})
	}
}

func BenchmarkKeyValueMap_String(b *testing.B) {
	b.Run("without_prefix", func(b *testing.B) {
		for _, size := range keySizes {
			b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
				m := createPopulatedMap(size)

				b.ResetTimer()
				b.ReportAllocs()

				for b.Loop() {
					_ = m.String()
				}
			})
		}
	})

	b.Run("with_prefix", func(b *testing.B) {
		for _, size := range keySizes {
			b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
				m := createPopulatedMap(size).WithPrefix("prefix")

				b.ResetTimer()
				b.ReportAllocs()

				for b.Loop() {
					_ = m.String()
				}
			})
		}
	})
}

func BenchmarkKeyValueMap_Range(b *testing.B) {
	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := createPopulatedMap(size)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				m.Range(func(k string, v any) {
					// Minimal work to avoid optimizing away the call
					_ = k
					_ = v
				})
			}
		})
	}
}

// Custom Stringer Benchmarks

func BenchmarkKeyValueMap_String_CustomStringer(b *testing.B) {
	customStringer := func(k string, v any) string {
		return fmt.Sprintf(`"%s":"%v"`, k, v) // JSON-like format
	}

	for _, size := range keySizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			m := NewKeyValueMap(".", ",", customStringer)
			for i := range size {
				m.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
			}

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = m.String()
			}
		})
	}
}

// Memory Usage Benchmarks

func BenchmarkKeyValueMap_MemoryUsage(b *testing.B) {
	b.Run("growth", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			m := NewKeyValueMap(".", " ", nil)
			for j := range 1000 {
				m.Set(strconv.Itoa(j), j)
			}
		}
	})

	b.Run("prefix_memory_overhead", func(b *testing.B) {
		baseMap := createPopulatedMap(100)
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = baseMap.WithPrefix("prefix").WithPrefix("nested")
		}
	})
}

// Concurrency Benchmarks

func BenchmarkKeyValueMap_ConcurrentReads(b *testing.B) {
	concurrencyLevels := []int{2, 4, 8, 16}

	for _, size := range keySizes {
		for _, concurrency := range concurrencyLevels {
			b.Run(fmt.Sprintf("size_%d_goroutines_%d", size, concurrency), func(b *testing.B) {
				m := createPopulatedMap(size)
				keys := generateKeys(size, "key")

				b.ResetTimer()
				b.SetParallelism(concurrency)

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						idx := runtime.NumGoroutine() % size
						if idx < 0 {
							idx = -idx
						}
						_, _ = m.Get(keys[idx])
					}
				})
			})
		}
	}
}

func BenchmarkKeyValueMap_ConcurrentWrites(b *testing.B) {
	concurrencyLevels := []int{2, 4, 8}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("goroutines_%d", concurrency), func(b *testing.B) {
			m := NewKeyValueMap(".", " ", nil)

			b.ResetTimer()
			b.SetParallelism(concurrency)

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					key := fmt.Sprintf("key_%d_%d", runtime.NumGoroutine(), i)
					m.Set(key, i)
					i++
				}
			})
		})
	}
}

func BenchmarkKeyValueMap_ConcurrentReadWrite(b *testing.B) {
	b.Run("mixed_operations", func(b *testing.B) {
		m := createPopulatedMap(1000)
		keys := generateKeys(1000, "key")

		b.ResetTimer()
		b.SetParallelism(8)

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%10 == 0 {
					// 10% writes
					key := fmt.Sprintf("new_key_%d_%d", runtime.NumGoroutine(), i)
					m.Set(key, i)
				} else {
					// 90% reads
					idx := i % len(keys)
					_, _ = m.Get(keys[idx])
				}
				i++
			}
		})
	})
}

// Specialized Logging Scenario Benchmarks

func BenchmarkKeyValueMap_LoggingScenarios(b *testing.B) {
	b.Run("typical_log_entry", func(b *testing.B) {
		// Simulates typical logging: create base context, add request-specific fields, serialize
		baseContext := NewKeyValueMap(".", " ", nil)
		baseContext.Set("service", "api")
		baseContext.Set("version", "1.0.0")
		baseContext.Set("env", "prod")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; b.Loop(); i++ {
			requestContext := baseContext.WithPairs(
				"request_id", fmt.Sprintf("req_%d", i),
				"user_id", i%1000,
				"endpoint", "/api/users",
				"method", "GET",
				"status", 200,
				"duration_ms", 150+i%100,
			)
			_ = requestContext.String()
		}
	})

	b.Run("nested_context", func(b *testing.B) {
		// Simulates nested contexts with prefixes (e.g., service.database.query)
		baseContext := createPopulatedMap(10)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; b.Loop(); i++ {
			dbContext := baseContext.WithPrefix("db")
			queryContext := dbContext.WithPrefix("query").WithPairs(
				"table", "users",
				"duration_ms", i%1000,
				"rows_affected", i%100,
			)
			_ = queryContext.String()
		}
	})

	b.Run("context_inheritance", func(b *testing.B) {
		// Simulates passing context through call chain with field additions
		rootContext := NewKeyValueMap(".", " ", nil)
		rootContext.Set("trace_id", "abc123")
		rootContext.Set("span_id", "def456")

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			// Service layer adds fields
			serviceContext := rootContext.WithPairs("service", "user-service")

			// Repository layer adds fields
			repoContext := serviceContext.WithPairs("repository", "user-repo", "query", "SELECT * FROM users")

			// Database layer adds fields
			dbContext := repoContext.WithPairs("db_host", "localhost", "db_name", "myapp")

			_ = dbContext.String()
		}
	})
}

// Edge Case Benchmarks

func BenchmarkKeyValueMap_EdgeCases(b *testing.B) {
	b.Run("empty_map_operations", func(b *testing.B) {
		m := NewKeyValueMap(".", " ", nil)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = m.String()
			_ = m.AsMap()
			_ = m.Len()
			_, _ = m.Get("non_existent")
		}
	})

	b.Run("large_values", func(b *testing.B) {
		largeValue := make([]byte, 1024) // 1KB value
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			m := NewKeyValueMap(".", " ", nil)
			m.Set("large_key", largeValue)
			_ = m.Clone()
		}
	})

	b.Run("deep_prefix_nesting", func(b *testing.B) {
		baseMap := createPopulatedMap(10)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			current := baseMap
			// Create deeply nested prefixes
			for j := range 10 {
				current = current.WithPrefix(fmt.Sprintf("level%d", j))
			}
			_ = current.String()
		}
	})
}

// Comparison Benchmarks (vs standard map)

func BenchmarkComparison_StandardMap(b *testing.B) {
	b.Run("standard_map_read", func(b *testing.B) {
		m := make(map[string]any, 1000)
		for i := range 1000 {
			m[fmt.Sprintf("key%d", i)] = i
		}

		var mu sync.RWMutex
		keys := generateKeys(1000, "key")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; b.Loop(); i++ {
			idx := i % 1000
			mu.RLock()
			_ = m[keys[idx]]
			mu.RUnlock()
		}
	})

	b.Run("keyvaluemap_read", func(b *testing.B) {
		m := createPopulatedMap(1000)
		keys := generateKeys(1000, "key")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; b.Loop(); i++ {
			idx := i % 1000
			_, _ = m.Get(keys[idx])
		}
	})
}

func BenchmarkComparison_Itoa(b *testing.B) {
	val := 123456789
	b.Run("fmt_sprint_int", func(b *testing.B) {
		for b.Loop() {
			_ = fmt.Sprint(val)
		}
	})
	b.Run("strconv_itoa", func(b *testing.B) {
		for b.Loop() {
			_ = strconv.Itoa(val)
		}
	})
	b.Run("keyvaluemap_itoa", func(b *testing.B) {
		for b.Loop() {
			_ = itoa64(int64(val))
		}
	})
}

func BenchmarkComparison_Uint64(b *testing.B) {
	val := uint64(9876543210)
	b.Run("fmt_sprint_uint64", func(b *testing.B) {
		for b.Loop() {
			_ = fmt.Sprint(val)
		}
	})
	b.Run("strconv_format_uint", func(b *testing.B) {
		for b.Loop() {
			_ = strconv.FormatUint(val, 10)
		}
	})
	b.Run("keyvaluemap_utoa64", func(b *testing.B) {
		for b.Loop() {
			_ = utoa64(val)
		}
	})
}
