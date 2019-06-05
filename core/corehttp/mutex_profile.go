package corehttp

import (
	"net"
	"net/http"
	"runtime"
	"strconv"

	core "github.com/ipfs/go-ipfs/core"
)

// MutexFractionOption allows to set runtime.SetMutexProfileFraction via HTTP
// using POST request with parameter 'fraction'.
func MutexFractionOption(path string) ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			asfr := r.Form.Get("fraction")
			if len(asfr) == 0 {
				http.Error(w, "parameter 'fraction' must be set", http.StatusBadRequest)
				return
			}

			fr, err := strconv.Atoi(asfr)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			log.Infof("Setting MutexProfileFraction to %d", fr)
			runtime.SetMutexProfileFraction(fr)
		})

		return mux, nil
	}
}
