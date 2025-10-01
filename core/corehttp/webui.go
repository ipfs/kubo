package corehttp

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
)

// WebUI version confirmed to work with this Kubo version
const WebUIPath = "/ipfs/bafybeicg7e6o2eszkfdzxg5233gmuip2a7kfzoloh7voyvt2r6ivdet54u" // v4.9.1

// WebUIPaths is a list of all past webUI paths.
var WebUIPaths = []string{
	WebUIPath,
	"/ipfs/bafybeifplj2s3yegn7ko7tdnwpoxa4c5uaqnk2ajnw5geqm34slcj6b6mu", // v4.8.0
	"/ipfs/bafybeibfd5kbebqqruouji6ct5qku3tay273g7mt24mmrfzrsfeewaal5y", // v4.7.0
	"/ipfs/bafybeibpaa5kqrj4gkemiswbwndjqiryl65cks64ypwtyerxixu56gnvvm", // v4.6.0
	"/ipfs/bafybeiata4qg7xjtwgor6r5dw63jjxyouenyromrrb4lrewxrlvav7gzgi", // v4.5.0
	"/ipfs/bafybeigp3zm7cqoiciqk5anlheenqjsgovp7j7zq6hah4nu6iugdgb4nby", // v4.4.2
	"/ipfs/bafybeiatztgdllxnp5p6zu7bdwhjmozsmd7jprff4bdjqjljxtylitvss4", // v4.4.1
	"/ipfs/bafybeibgic2ex3fvzkinhy6k6aqyv3zy2o7bkbsmrzvzka24xetv7eeadm", // v4.4.0
	"/ipfs/bafybeid4uxz7klxcu3ffsnmn64r7ihvysamlj4ohl5h2orjsffuegcpaeq", // v4.3.3
	"/ipfs/bafybeif6abowqcavbkz243biyh7pde7ick5kkwwytrh7pd2hkbtuqysjxy", // v4.3.2
	"/ipfs/bafybeihatzsgposbr3hrngo42yckdyqcc56yean2rynnwpzxstvdlphxf4",
	"/ipfs/bafybeigggyffcf6yfhx5irtwzx3cgnk6n3dwylkvcpckzhqqrigsxowjwe",
	"/ipfs/bafybeidf7cpkwsjkq6xs3r6fbbxghbugilx3jtezbza7gua3k5wjixpmba",
	"/ipfs/bafybeiamycmd52xvg6k3nzr6z3n33de6a2teyhquhj4kspdtnvetnkrfim",
	"/ipfs/bafybeieqdeoqkf7xf4aozd524qncgiloh33qgr25lyzrkusbcre4c3fxay",
	"/ipfs/bafybeicyp7ssbnj3hdzehcibmapmpuc3atrsc4ch3q6acldfh4ojjdbcxe",
	"/ipfs/bafybeigs6d53gpgu34553mbi5bbkb26e4ikruoaaar75jpfdywpup2r3my",
	"/ipfs/bafybeic4gops3d3lyrisqku37uio33nvt6fqxvkxihrwlqsuvf76yln4fm",
	"/ipfs/bafybeifeqt7mvxaniphyu2i3qhovjaf3sayooxbh5enfdqtiehxjv2ldte", // v2.22.0
	"/ipfs/bafybeiequgo72mrvuml56j4gk7crewig5bavumrrzhkqbim6b3s2yqi7ty",
	"/ipfs/bafybeibjbq3tmmy7wuihhhwvbladjsd3gx3kfjepxzkq6wylik6wc3whzy", // v2.20.0
	"/ipfs/bafybeiavrvt53fks6u32n5p2morgblcmck4bh4ymf4rrwu7ah5zsykmqqa", // v2.19.0
	"/ipfs/bafybeiageaoxg6d7npaof6eyzqbwvbubyler7bq44hayik2hvqcggg7d2y", // v2.18.1
	"/ipfs/bafybeidb5eryh72zajiokdggzo7yct2d6hhcflncji5im2y5w26uuygdsm", // v2.18.0
	"/ipfs/bafybeibozpulxtpv5nhfa2ue3dcjx23ndh3gwr5vwllk7ptoyfwnfjjr4q", // v2.15.1
	"/ipfs/bafybeiednzu62vskme5wpoj4bjjikeg3xovfpp4t7vxk5ty2jxdi4mv4bu", // v2.15.0
	"/ipfs/bafybeihcyruaeza7uyjd6ugicbcrqumejf6uf353e5etdkhotqffwtguva", // v2.13.0
	"/ipfs/bafybeiflkjt66aetfgcrgvv75izymd5kc47g6luepqmfq6zsf5w6ueth6y",
	"/ipfs/bafybeid26vjplsejg7t3nrh7mxmiaaxriebbm4xxrxxdunlk7o337m5sqq",
	"/ipfs/bafybeif4zkmu7qdhkpf3pnhwxipylqleof7rl6ojbe7mq3fzogz6m4xk3i", // v2.11.4
	"/ipfs/bafybeianwe4vy7sprht5sm3hshvxjeqhwcmvbzq73u55sdhqngmohkjgs4",
	"/ipfs/bafybeicitin4p7ggmyjaubqpi3xwnagrwarsy6hiihraafk5rcrxqxju6m",
	"/ipfs/bafybeihpetclqvwb4qnmumvcn7nh4pxrtugrlpw4jgjpqicdxsv7opdm6e",
	"/ipfs/bafybeibnnxd4etu4tq5fuhu3z5p4rfu3buabfkeyr3o3s4h6wtesvvw6mu",
	"/ipfs/bafybeid6luolenf4fcsuaw5rgdwpqbyerce4x3mi3hxfdtp5pwco7h7qyq",
	"/ipfs/bafybeigkbbjnltbd4ewfj7elajsbnjwinyk6tiilczkqsibf3o7dcr6nn4",
	"/ipfs/bafybeicp23nbcxtt2k2twyfivcbrc6kr3l5lnaiv3ozvwbemtrb7v52r6i",
	"/ipfs/bafybeidatpz2hli6fgu3zul5woi27ujesdf5o5a7bu622qj6ugharciwjq",
	"/ipfs/QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	"/ipfs/QmXc9raDM1M5G5fpBnVyQ71vR4gbnskwnB9iMEzBuLgvoZ",
	"/ipfs/QmenEBWcAk3tN94fSKpKFtUMwty1qNwSYw3DMDFV6cPBXA",
	"/ipfs/QmUnXcWZC5Ve21gUseouJsH5mLAyz5JPp8aHsg8qVUUK8e",
	"/ipfs/QmSDgpiHco5yXdyVTfhKxr3aiJ82ynz8V14QcGKicM3rVh",
	"/ipfs/QmRuvWJz1Fc8B9cTsAYANHTXqGmKR9DVfY5nvMD1uA2WQ8",
	"/ipfs/QmQLXHs7K98JNQdWrBB2cQLJahPhmupbDjRuH1b9ibmwVa",
	"/ipfs/QmXX7YRpU7nNBKfw75VG7Y1c3GwpSAGHRev67XVPgZFv9R",
	"/ipfs/QmXdu7HWdV6CUaUabd9q2ZeA4iHZLVyDRj3Gi4dsJsWjbr",
	"/ipfs/QmaaqrHyAQm7gALkRW8DcfGX3u8q9rWKnxEMmf7m9z515w",
	"/ipfs/QmSHDxWsMPuJQKWmVA1rB5a3NX2Eme5fPqNb63qwaqiqSp",
	"/ipfs/QmctngrQAt9fjpQUZr7Bx3BsXUcif52eZGTizWhvcShsjz",
	"/ipfs/QmS2HL9v5YeKgQkkWMvs1EMnFtUowTEdFfSSeMT4pos1e6",
	"/ipfs/QmR9MzChjp1MdFWik7NjEjqKQMzVmBkdK3dz14A6B5Cupm",
	"/ipfs/QmRyWyKWmphamkMRnJVjUTzSFSAAZowYP4rnbgnfMXC9Mr",
	"/ipfs/QmU3o9bvfenhTKhxUakbYrLDnZU7HezAVxPM6Ehjw9Xjqy",
	"/ipfs/QmPhnvn747LqwPYMJmQVorMaGbMSgA7mRRoyyZYz3DoZRQ",
	"/ipfs/QmQNHd1suZTktPRhP7DD4nKWG46ZRSxkwHocycHVrK3dYW",
	"/ipfs/QmNyMYhwJUS1cVvaWoVBhrW8KPj1qmie7rZcWo8f1Bvkhz",
	"/ipfs/QmVTiRTQ72qiH4usAGT4c6qVxCMv4hFMUH9fvU6mktaXdP",
	"/ipfs/QmYcP4sp1nraBiCYi6i9kqdaKobrK32yyMpTrM5JDA8a2C",
	"/ipfs/QmUtMmxgHnDvQq4bpH6Y9MaLN1hpfjJz5LZcq941BEqEXs",
	"/ipfs/QmPURAjo3oneGH53ovt68UZEBvsc8nNmEhQZEpsVEQUMZE",
	"/ipfs/QmeSXt32frzhvewLKwA1dePTSjkTfGVwTh55ZcsJxrCSnk",
	"/ipfs/QmcjeTciMNgEBe4xXvEaA4TQtwTRkXucx7DmKWViXSmX7m",
	"/ipfs/QmfNbSskgvTXYhuqP8tb9AKbCkyRcCy3WeiXwD9y5LeoqK",
	"/ipfs/QmPkojhjJkJ5LEGBDrAvdftrjAYmi9GU5Cq27mWvZTDieW",
	"/ipfs/Qmexhq2sBHnXQbvyP2GfUdbnY7HCagH2Mw5vUNSBn2nxip",
}

// WebUIOption provides the WebUI handler for the RPC API.
func WebUIOption(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	handler := &webUIHandler{
		headers:               cfg.API.HTTPHeaders,
		node:                  n,
		noFetch:               cfg.Gateway.NoFetch,
		deserializedResponses: cfg.Gateway.DeserializedResponses.WithDefault(config.DefaultDeserializedResponses),
	}

	mux.Handle("/webui/", handler)
	return mux, nil
}

type webUIHandler struct {
	headers               map[string][]string
	node                  *core.IpfsNode
	noFetch               bool
	deserializedResponses bool
}

func (h *webUIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for k, v := range h.headers {
		w.Header()[http.CanonicalHeaderKey(k)] = v
	}

	// Check if WebUI is incompatible with current configuration
	if !h.deserializedResponses {
		h.writeIncompatibleError(w)
		return
	}

	// Check if WebUI is available locally when Gateway.NoFetch is true
	if h.noFetch {
		cidStr := strings.TrimPrefix(WebUIPath, "/ipfs/")
		webUICID, err := cid.Parse(cidStr)
		if err != nil {
			// This should never happen with hardcoded constant
			log.Errorf("failed to parse WebUI CID: %v", err)
		} else {
			has, err := h.node.Blockstore.Has(r.Context(), webUICID)
			if err != nil {
				log.Debugf("error checking WebUI availability: %v", err)
			} else if !has {
				h.writeNotAvailableError(w)
				return
			}
		}
	}

	// Default behavior: redirect to the WebUI path
	http.Redirect(w, r, WebUIPath, http.StatusFound)
}

func (h *webUIHandler) writeIncompatibleError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, `IPFS WebUI Incompatible

WebUI is not compatible with Gateway.DeserializedResponses=false.

The WebUI requires deserializing IPFS responses to render the interface.
To use the WebUI, set Gateway.DeserializedResponses=true in your config.
`)
}

func (h *webUIHandler) writeNotAvailableError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, `IPFS WebUI Not Available

WebUI at %s is not in your local node due to Gateway.NoFetch=true.

To use the WebUI, either:
1. Run: ipfs pin add --progress --name ipfs-webui %s
2. Download from https://github.com/ipfs/ipfs-webui/releases and import with: ipfs dag import ipfs-webui.car
`, WebUIPath, WebUIPath)
}
