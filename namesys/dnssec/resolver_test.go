package dnssec

import (
	"context"
	"fmt"
)

func ExampleResolver_LookupTXT() {
	r := &Resolver{}

	txts, res, err := r.LookupTXT(context.Background(), "dnssec.brendans.website")
	if err != nil {
		panic(err)
	}
	fmt.Println(txts)
	fmt.Println(res.Verify())
	fmt.Println(res.TXT("dnssec.brendans.website"))
	fmt.Println(res.TXT("wrong-zone.com"))

	// Output:
	// [secure txt record]
	// <nil>
	// [secure txt record] <nil>
	// [] unexpected record name: dnssec.brendans.website.
}
