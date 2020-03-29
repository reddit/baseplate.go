package randbp_test

import (
	crand "crypto/rand"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/randbp"
)

func swap(slice []byte) func(i, j int) {
	return func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func seedBoth() {
	seed := randbp.GetSeed()
	rand.Seed(seed)
	randbp.R.Seed(seed)
}

func TestSameResult(t *testing.T) {
	seedBoth()

	const epsilon = 1e-9
	equalFloat64 := func(a, b float64) bool {
		return math.Abs(a-b) < epsilon
	}

	t.Run(
		"Uint64",
		func(t *testing.T) {
			f := func() bool {
				u64math := rand.Uint64()
				u64bp := randbp.R.Uint64()
				if u64math != u64bp {
					t.Errorf(
						"math/rand.Uint64() returned %d while randbp.R.Uint64() returned %d",
						u64math,
						u64bp,
					)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"Float64",
		func(t *testing.T) {
			f := func() bool {
				f64math := rand.Float64()
				f64bp := randbp.R.Float64()
				if !equalFloat64(f64math, f64bp) {
					t.Errorf(
						"math/rand.Float64() returned %v while randbp.R.Float64() returned %v",
						f64math,
						f64bp,
					)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"NormFloat64",
		func(t *testing.T) {
			f := func() bool {
				f64math := rand.NormFloat64()
				f64bp := randbp.R.NormFloat64()
				if !equalFloat64(f64math, f64bp) {
					t.Errorf(
						"math/rand.NormFloat64() returned %v while randbp.R.NormFloat64() returned %v",
						f64math,
						f64bp,
					)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"ExpFloat64",
		func(t *testing.T) {
			f := func() bool {
				f64math := rand.ExpFloat64()
				f64bp := randbp.R.ExpFloat64()
				if !equalFloat64(f64math, f64bp) {
					t.Errorf(
						"math/rand.ExpFloat64() returned %v while randbp.R.ExpFloat64() returned %v",
						f64math,
						f64bp,
					)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"Shuffle",
		func(t *testing.T) {
			f := func(s string) bool {
				mathSlice := make([]byte, len(s))
				bpSlice := make([]byte, len(s))
				copy(mathSlice, []byte(s))
				copy(bpSlice, []byte(s))
				rand.Shuffle(len(s), swap(mathSlice))
				randbp.R.Shuffle(len(s), swap(bpSlice))
				if !reflect.DeepEqual(mathSlice, bpSlice) {
					t.Errorf(
						"math/rand.Shuffle() returned %x while randbp.R.Shuffle() returned %x",
						mathSlice,
						bpSlice,
					)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"Perm",
		func(t *testing.T) {
			f := func(smalln uint8) bool {
				n := int(smalln)
				mathSlice := rand.Perm(n)
				bpSlice := randbp.R.Perm(n)
				if !reflect.DeepEqual(mathSlice, bpSlice) {
					t.Errorf(
						"math/rand.Shuffle() returned %v while randbp.R.Shuffle() returned %v",
						mathSlice,
						bpSlice,
					)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)
}

func BenchmarkRand(b *testing.B) {
	sizes := []int{16, 64, 256, 512, 1024, 4096, 1024 * 1024}
	seedBoth()

	b.Run(
		"Read",
		func(b *testing.B) {
			for _, size := range sizes {
				b.Run(
					fmt.Sprintf("size-%d", size),
					func(b *testing.B) {
						b.Run(
							"math/rand",
							func(b *testing.B) {
								b.RunParallel(func(pb *testing.PB) {
									buf := make([]byte, size)
									for pb.Next() {
										rand.Read(buf)
									}
								})
							},
						)

						b.Run(
							"crypto/rand",
							func(b *testing.B) {
								b.RunParallel(func(pb *testing.PB) {
									buf := make([]byte, size)
									for pb.Next() {
										if _, err := crand.Read(buf); err != nil {
											b.Fatal(err)
										}
									}
								})
							},
						)

						b.Run(
							"randbp",
							func(b *testing.B) {
								b.RunParallel(func(pb *testing.PB) {
									buf := make([]byte, size)
									for pb.Next() {
										randbp.R.Read(buf)
									}
								})
							},
						)
					},
				)
			}
		},
	)

	b.Run(
		"Uint64",
		func(b *testing.B) {
			b.Run(
				"math/rand",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							rand.Uint64()
						}
					})
				},
			)

			b.Run(
				"randbp",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							randbp.R.Uint64()
						}
					})
				},
			)
		},
	)

	b.Run(
		"Float64",
		func(b *testing.B) {
			b.Run(
				"math/rand",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							rand.Float64()
						}
					})
				},
			)

			b.Run(
				"randbp",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							randbp.R.Float64()
						}
					})
				},
			)
		},
	)

	b.Run(
		"NormFloat64",
		func(b *testing.B) {
			b.Run(
				"math/rand",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							rand.NormFloat64()
						}
					})
				},
			)

			b.Run(
				"randbp",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							randbp.R.NormFloat64()
						}
					})
				},
			)
		},
	)

	b.Run(
		"ExpFloat64",
		func(b *testing.B) {
			b.Run(
				"math/rand",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							rand.ExpFloat64()
						}
					})
				},
			)

			b.Run(
				"randbp",
				func(b *testing.B) {
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							randbp.R.ExpFloat64()
						}
					})
				},
			)
		},
	)

	b.Run(
		"Shuffle",
		func(b *testing.B) {
			for _, size := range sizes {
				b.Run(
					fmt.Sprintf("size-%d", size),
					func(b *testing.B) {
						b.Run(
							"math/rand",
							func(b *testing.B) {
								slice := make([]byte, size)
								b.RunParallel(func(pb *testing.PB) {
									for pb.Next() {
										rand.Shuffle(size, swap(slice))
									}
								})
							},
						)

						b.Run(
							"randbp",
							func(b *testing.B) {
								slice := make([]byte, size)
								b.RunParallel(func(pb *testing.PB) {
									for pb.Next() {
										randbp.R.Shuffle(size, swap(slice))
									}
								})
							},
						)
					},
				)
			}
		},
	)

	b.Run(
		"Perm",
		func(b *testing.B) {
			for _, size := range sizes {
				b.Run(
					fmt.Sprintf("size-%d", size),
					func(b *testing.B) {
						b.Run(
							"math/rand",
							func(b *testing.B) {
								b.RunParallel(func(pb *testing.PB) {
									for pb.Next() {
										rand.Perm(size)
									}
								})
							},
						)

						b.Run(
							"randbp",
							func(b *testing.B) {
								b.RunParallel(func(pb *testing.PB) {
									for pb.Next() {
										randbp.R.Perm(size)
									}
								})
							},
						)
					},
				)
			}
		},
	)
}
