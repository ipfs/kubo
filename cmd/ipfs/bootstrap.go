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
	"os"
//	"io"
"bufio"
	
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

                fmt.Println(conf.Bootstrap[i])
		 	    s := []string{conf.Bootstrap[i].Address, "/", conf.Bootstrap[i].PeerID, "\n"}
		 	     fmt.Printf(strings.Join(s, ""))
		 	}

		return nil
		
	    }
		
		
	  switch arg := inp[0]; arg {
	      case "add":
			  if len(inp) == 1 {
				  fmt.Println("No peer specified.")
				  return nil
			  }
			  
			  

			  	var stringArr = strings.SplitAfterN(inp[1], "/", 6)
		 	    s := []string{stringArr[0], stringArr[1], stringArr[2], stringArr[3], stringArr[4]}
				var peerID = stringArr[5]
                var address = strings.Join(s, "")
			
				
		     	fmt.Printf("the peer ID is : %q\n the address is: %q\n", peerID, address)
			   
  
  
				//bootstrap object created of user entered peer data
  				peer := Peer{
	  				  		PeerID:    peerID,
	  					  	Address:   address,


	  				}
					
 				
					
	// 				peers := []Peer{peer}
	// 				 config := Config {
	//  						Bootstrap: peers,
	//  					}
	//
	//
	// 				b, err := json.Marshal(config)
	// 	  			os.Stdout.Write(b)
	// 				if(err != nil) {
	// 					panic(err)
	// 				}
	//
	
					b, err := json.Marshal(peer)
					if (err != nil) {
						panic(err)
					}
					
					configpath, _ := u.TildeExpansion("~/.go-ipfs/config")
					
					err := AddPeertoFile(configpath, b)
					if(err != nil) {
						panic(err)
					}
					
			  fmt.Println(inp[1])
			  
			  
			  
			  return nil
	      case "remove":
			  if len(inp) == 1 {
				  fmt.Println("No peer specified.")
				  return nil
			  }
			  
			  //TODO remove the peer from the config file 
			  //1 find the peer that matches
			  //delete that peer from the file (iout)
			  fmt.Println(inp[1])
			  
			  
			  return nil
	  }
	
	  return nil

 }
 
 func AddPeertoFile(filename string, peerData []byte) error {
     // open the file
     file, err := os.Open(filename)
     if err != nil {
         return err
     }
     // get the file permissions (for later)
     info, err := file.Stat()
     if err != nil {
         return err
     }
     perm := info.Mode()
     // read the file line by line
     lines := []string{}
     for scanner := bufio.NewScanner(file); scanner.Scan(); {
         lines = append(lines, scanner.Text())
     }
     // close the file
     if err = file.Close(); err != nil {
         return err
     }
    
	
	 //find line with ] 
	 for i, line := range lines {
		 if(strings.ContainsRune(line, ']')) {
			 //take the line before... and append/write to it
			 lines[i-1]
			 
		 }
	 }
	
	
     
     // O_TRUNC will truncate the file upon open
     file, err = os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, perm)
     if err != nil {
         return err
     }
     defer file.Close()
     // write the lines back to the file
     for _, line := range lines {
         if _, err = file.WriteString(line + "\n"); err != nil {
             return err
         }
     }
     return nil
 }

