package daemon

import (
	"fmt"
	"testing"
)

func TestCommandCall(t *testing.T) {
	dl, err := NewDaemonListener("localhost:12345")
	if err != nil {
		t.Fatal(err)
	}

	go dl.Listen()
	defer dl.Close()

	go func() {
		_, err := SendCommand("test command for fun", "localhost:12345")
		if err != nil {
			t.Fatal(err)
		}
	}()

	cmd := <-dl.CommChan
	if cmd.command != "test" {
		t.Fatal("command parsing failed.")
	}

	if cmd.args[0] != "command" ||
		cmd.args[1] != "for" ||
		cmd.args[2] != "fun" {
		t.Fatal("Args parsed incorrectly.")
	}
}

func TestFailures(t *testing.T) {
	dl, err := NewDaemonListener("localhost:12345")
	if err != nil {
		t.Fatal(err)
	}

	go dl.Listen()
	defer dl.Close()

	go func() {
		_, err := SendCommand("test", "localhost:12345")
		if err != nil {
			t.Fatal(err)
		}
	}()

	cmd := <-dl.CommChan
	if cmd.command != "test" || len(cmd.args) > 0 {
		t.Fatal("Parsing Failed.")
	}

	go func() {
		_, err := SendCommand("", "localhost:12345")
		if err != nil {
			t.Fatal(err)
		}
	}()

	cmd = <-dl.CommChan
	if cmd.command != "" || len(cmd.args) > 0 {
		fmt.Println(cmd)
		t.Fatal("Parsing Failed.")
	}

}
