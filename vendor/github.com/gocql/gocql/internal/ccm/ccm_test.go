// +build ccm

package ccm

import (
	"testing"
)

func TestCCM(t *testing.T) {
	if err := AllUp(); err != nil {
		t.Fatal(err)
	}

	status, err := Status()
	if err != nil {
		t.Fatal(err)
	}

	if host, ok := status["node1"]; !ok {
		t.Fatal("node1 not in status list")
	} else if !host.State.IsUp() {
		t.Fatal("node1 is not up")
	}

	NodeDown("node1")
	status, err = Status()
	if err != nil {
		t.Fatal(err)
	}

	if host, ok := status["node1"]; !ok {
		t.Fatal("node1 not in status list")
	} else if host.State.IsUp() {
		t.Fatal("node1 is not down")
	}

	NodeUp("node1")
	status, err = Status()
	if err != nil {
		t.Fatal(err)
	}

	if host, ok := status["node1"]; !ok {
		t.Fatal("node1 not in status list")
	} else if !host.State.IsUp() {
		t.Fatal("node1 is not up")
	}
}
