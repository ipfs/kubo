package commands

import (
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/multiformats/go-multibase"
)

func TestCidFmtCmd(t *testing.T) {
	t.Parallel()

	// Test 'error when -v 0 is present and a custom -b is passed'
	t.Run("ipfs cid format <cid> -b z -v 0", func(t *testing.T) {
		t.Parallel()

		type testV0PresentAndCustomBaseCase struct {
			MultibaseName  string
			ExpectedErrMsg string
		}

		var testV0PresentAndCustomBaseCases []testV0PresentAndCustomBaseCase

		for _, e := range multibase.EncodingToStr {
			var testCase testV0PresentAndCustomBaseCase

			if e == "base58btc" {
				testCase.MultibaseName = e
				testCase.ExpectedErrMsg = ""
				testV0PresentAndCustomBaseCases = append(testV0PresentAndCustomBaseCases, testCase)
				continue
			}
			testCase.MultibaseName = e
			testCase.ExpectedErrMsg = "cannot convert to CIDv0 with any multibase other than the implicit base58btc"
			testV0PresentAndCustomBaseCases = append(testV0PresentAndCustomBaseCases, testCase)
		}

		for _, e := range testV0PresentAndCustomBaseCases {

			// Mock request
			req := &cmds.Request{
				Options: map[string]interface{}{
					cidVerisonOptionName:   "0",
					cidMultibaseOptionName: e.MultibaseName,
					cidFormatOptionName:    "%s",
				},
			}

			// Response emitter
			resp := cmds.ResponseEmitter(nil)

			// Call the CidFmtCmd function with the mock request and response
			err := cidFmtCmd.Run(req, resp, nil)
			if err == nil && e.MultibaseName == "base58btc" {
				continue
			}

			errMsg := err.Error()
			if errMsg != e.ExpectedErrMsg {
				t.Errorf("Expected %s, got %s instead", e.ExpectedErrMsg, errMsg)
			}
		}
	})

	// Test 'upgrade CID to v1 when passing a custom -b and no -v is specified'
	t.Run("ipfs cid format <cid-version-0> -b z", func(t *testing.T) {
		t.Parallel()

		type testImplicitVersionAndCustomMultibaseCase struct {
			Ver           string
			CidV1         string
			CidV0         string
			MultibaseName string
		}

		var testCases = []testImplicitVersionAndCustomMultibaseCase{
			{
				Ver:           "",
				CidV1:         "zdj7WWwMSWGoyxYkkT7mHgYvr6tV8CYd77aYxxqSbg9HsiMcE",
				CidV0:         "QmPr755CxWUwt39C2Yiw4UGKrv16uZhSgeZJmoHUUS9TSJ",
				MultibaseName: "z",
			},
			{
				Ver:           "",
				CidV1:         "CAFYBEIDI7ZABPGG3S63QW3AJG2XAZNE4NJQPN777WLWYRAIDG3TE5QFN3A======",
				CidV0:         "QmVQVyEijmLb2cBQrowNQsaPbnUnJhfDK1sYe3wepm6ySf",
				MultibaseName: "base32padupper",
			},
		}
		for _, e := range testCases {
			// Mock request
			req := &cmds.Request{
				Options: map[string]interface{}{
					cidVerisonOptionName:   e.Ver,
					cidMultibaseOptionName: e.MultibaseName,
					cidFormatOptionName:    "%s",
				},
			}

			// Response emitter
			resp := cmds.ResponseEmitter(nil)

			// Call the CidFmtCmd function with the mock request and response
			err := cidFmtCmd.Run(req, resp, nil)

			if err != nil {
				t.Error(err)
			}
		}
	})
}
