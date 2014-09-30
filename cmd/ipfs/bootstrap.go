package main

import (

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	//"github.com/jbenet/go-ipfs/core/commands"
	"fmt"
    "io/ioutil"
   "encoding/json"
	u "github.com/jbenet/go-ipfs/util"
    "strings"
	
	
	
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



func bootstrapCmd(c *commander.Command, inp []string) error {
	
	if len(inp) == 0 {
		
		configpath, _ := u.TildeExpansion("~/.go-ipfs/config")
		    dat, _ := ioutil.ReadFile(configpath)
		    var configText = string(dat)


		 var conf Config
		  	 err := json.Unmarshal([]byte(configText), &conf)

		 	 if err != nil {
		 		fmt.Print("Error:", err)
		 	 }


		 	 //printing list of peers
		 	for i, _ := range conf.Bootstrap {

		 	    s := []string{conf.Bootstrap[i].Address, "/", conf.Bootstrap[i].PeerID, "\n"}
		 	     fmt.Printf(strings.Join(s, ""))
		 	}

		return nil
		
	    }
		
		
	  switch arg := inp[0]; arg {
	      case "add":
			  fmt.Println("YO")
			  return nil
	      case "remove":
			  fmt.Println("YO")
			  return nil
	  }
	
	  return nil

 }

