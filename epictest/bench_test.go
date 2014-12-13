package epictest

import "testing"

func benchmarkAddCat(conf Config, b *testing.B) {
	b.SetBytes(conf.DataAmountBytes)
	for n := 0; n < b.N; n++ {
		if err := AddCatBytes(conf); err != nil {
			b.Fatal(err)
		}
	}
}

var instant = Config{}.All_Instantaneous()

func BenchmarkInstantaneousAddCat1MB(b *testing.B)   { benchmarkAddCat(instant.Megabytes(1), b) }
func BenchmarkInstantaneousAddCat2MB(b *testing.B)   { benchmarkAddCat(instant.Megabytes(2), b) }
func BenchmarkInstantaneousAddCat4MB(b *testing.B)   { benchmarkAddCat(instant.Megabytes(4), b) }
func BenchmarkInstantaneousAddCat8MB(b *testing.B)   { benchmarkAddCat(instant.Megabytes(8), b) }
func BenchmarkInstantaneousAddCat16MB(b *testing.B)  { benchmarkAddCat(instant.Megabytes(16), b) }
func BenchmarkInstantaneousAddCat32MB(b *testing.B)  { benchmarkAddCat(instant.Megabytes(32), b) }
func BenchmarkInstantaneousAddCat64MB(b *testing.B)  { benchmarkAddCat(instant.Megabytes(64), b) }
func BenchmarkInstantaneousAddCat128MB(b *testing.B) { benchmarkAddCat(instant.Megabytes(128), b) }
func BenchmarkInstantaneousAddCat256MB(b *testing.B) { benchmarkAddCat(instant.Megabytes(256), b) }

var routing = Config{}.Routing_Slow()

func BenchmarkRoutingSlowAddCat1MB(b *testing.B)  { benchmarkAddCat(routing.Megabytes(1), b) }
func BenchmarkRoutingSlowAddCat2MB(b *testing.B)  { benchmarkAddCat(routing.Megabytes(2), b) }
func BenchmarkRoutingSlowAddCat4MB(b *testing.B)  { benchmarkAddCat(routing.Megabytes(4), b) }
func BenchmarkRoutingSlowAddCat8MB(b *testing.B)  { benchmarkAddCat(routing.Megabytes(8), b) }
func BenchmarkRoutingSlowAddCat16MB(b *testing.B) { benchmarkAddCat(routing.Megabytes(16), b) }
func BenchmarkRoutingSlowAddCat32MB(b *testing.B) { benchmarkAddCat(routing.Megabytes(32), b) }

var network = Config{}.Network_NYtoSF()

func BenchmarkNetworkSlowAddCat1MB(b *testing.B)   { benchmarkAddCat(network.Megabytes(1), b) }
func BenchmarkNetworkSlowAddCat2MB(b *testing.B)   { benchmarkAddCat(network.Megabytes(2), b) }
func BenchmarkNetworkSlowAddCat4MB(b *testing.B)   { benchmarkAddCat(network.Megabytes(4), b) }
func BenchmarkNetworkSlowAddCat8MB(b *testing.B)   { benchmarkAddCat(network.Megabytes(8), b) }
func BenchmarkNetworkSlowAddCat16MB(b *testing.B)  { benchmarkAddCat(network.Megabytes(16), b) }
func BenchmarkNetworkSlowAddCat32MB(b *testing.B)  { benchmarkAddCat(network.Megabytes(32), b) }
func BenchmarkNetworkSlowAddCat64MB(b *testing.B)  { benchmarkAddCat(network.Megabytes(64), b) }
func BenchmarkNetworkSlowAddCat128MB(b *testing.B) { benchmarkAddCat(network.Megabytes(128), b) }
func BenchmarkNetworkSlowAddCat256MB(b *testing.B) { benchmarkAddCat(network.Megabytes(256), b) }

var blockstore = Config{}.Blockstore_7200RPM()

func BenchmarkBlockstoreSlowAddCat1MB(b *testing.B)   { benchmarkAddCat(blockstore.Megabytes(1), b) }
func BenchmarkBlockstoreSlowAddCat2MB(b *testing.B)   { benchmarkAddCat(blockstore.Megabytes(2), b) }
func BenchmarkBlockstoreSlowAddCat4MB(b *testing.B)   { benchmarkAddCat(blockstore.Megabytes(4), b) }
func BenchmarkBlockstoreSlowAddCat8MB(b *testing.B)   { benchmarkAddCat(blockstore.Megabytes(8), b) }
func BenchmarkBlockstoreSlowAddCat16MB(b *testing.B)  { benchmarkAddCat(blockstore.Megabytes(16), b) }
func BenchmarkBlockstoreSlowAddCat32MB(b *testing.B)  { benchmarkAddCat(blockstore.Megabytes(32), b) }
func BenchmarkBlockstoreSlowAddCat64MB(b *testing.B)  { benchmarkAddCat(blockstore.Megabytes(64), b) }
func BenchmarkBlockstoreSlowAddCat128MB(b *testing.B) { benchmarkAddCat(blockstore.Megabytes(128), b) }
func BenchmarkBlockstoreSlowAddCat256MB(b *testing.B) { benchmarkAddCat(blockstore.Megabytes(256), b) }
