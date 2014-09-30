package main

import (

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	//"github.com/jbenet/go-ipfs/core/commands"
	"fmt"
    "io/ioutil"
    "encoding/json"
	
	
)


var cmdIpfsBootstrap = &commander.Command{
	UsageLine: "bootstrap",
	Short:     "Show a list of bootsrapped addresses.",
	Long: `ipfs bootstrap <add/remove> <address>... - show/add/remove bootstrapped addresses

	Shows a list of bootstrapped addresses. use 'add' or 'remove' followed
	by a specified <address> to add/remove it from the list.
`,
	Run:  bootstrapCmd,
	Flag: *flag.NewFlagSet("ipfs-bootstrap", flag.ExitOnError),
}


type Peer struct {
	PeerID     string
	Address string
}

type Config struct {
	Bootstrap []Peer
}

var in = `{
    "peers": [
        {
            "pid": 1,
            "address": "127.0.0.1:17001"
        },
        {
            "pid": 2,
            "address": "127.0.0.1:17002"
        }
    ]
}`



func bootstrapCmd(c *commander.Command, inp []string) error {
	
	
    dat, _ := ioutil.ReadFile("$HOME/.go-ipfs/config")
    var configText = string(dat)
	
	
	 var conf Config
 	 err := json.Unmarshal([]byte(configText), &conf)

	 if err != nil {
		fmt.Print("Error:", err)
	 }

 	fmt.Printf("%#v\n", conf)
	fmt.Printf("%#v\n", conf.Bootstrap[0].PeerID)
	return nil

 }
//
// func (q *JSONData) FromJSON(file string) error {
//
//     //Reading JSON file
//     J, err := ioutil.ReadFile(file)
//     if err != nil {
// 		panic(err)
// 	}
//
//
//     var data = &q
//     //Umarshalling JSON into struct
//     return json.Unmarshal(J, data)
// }
