// A simple client for connecting and interacting with redis.
//
// To import inside your package do:
//
//	import "github.com/fzzy/radix/redis"
//
// Connecting
//
// Use either Dial or DialTimeout:
//
//	client, err := redis.Dial("tcp", "localhost:6379")
//	if err != nil {
//		// handle err
//	}
//
// Make sure to call Close on the client if you want to clean it up before the
// end of the program.
//
// Cmd and Reply
//
// The Cmd method returns a Reply, which has methods for converting to various
// types. Each of these methods returns an error which can either be a
// connection error (e.g. timeout), an application error (e.g. key is wrong
// type), or a conversion error (e.g. cannot convert to integer). You can also
// directly check the error using the Err field:
//
//	foo, err := client.Cmd("GET", "foo").Str()
//	if err != nil {
//		// handle err
//	}
//
//	// Checking Err field directly
//
//	err = client.Cmd("PING").Err
//	if err != nil {
//		// handle err
//	}
//
// Multi Replies
//
// The elements to Multi replies can be accessed as strings using List or
// ListBytes, or you can use the Elems field for more low-level access:
//
//	r := client.Cmd("MGET", "foo", "bar", "baz")
//
//	// This:
//	for _, elemStr := range r.List() {
//		fmt.Println(elemStr)
//	}
//
//	// is equivalent to this:
//	for i := range r.Elems {
//		elemStr, _ := r.Elems[i].Str()
//		fmt.Println(elemStr)
//	}
//
// Pipelining
//
// Pipelining is when the client sends a bunch of commands to the server at
// once, and only once all the commands have been sent does it start reading the
// replies off the socket. This is supported using the Append and GetReply
// methods. Append will simply append the command to a buffer without sending
// it, the first time GetReply is called it will send all the commands in the
// buffer and return the Reply for the first command that was sent. Subsequent
// calls to GetReply return Replys for subsequent commands:
//
//	client.Append("GET", "foo")
//	client.Append("SET", "bar", "foo")
//	client.Append("DEL", "baz")
//
//	// Read GET foo reply
//	foo, err := client.GetReply().Str()
//	if err != nil {
//		// handle err
//	}
//
//	// Read SET bar foo reply
//	if err := client.GetReply().Err; err != nil {
//		// handle err
//	}
//
//	// Read DEL baz reply
//	if err := client.GetReply().Err; err != nil {
//		// handle err
//	}
//
package redis
