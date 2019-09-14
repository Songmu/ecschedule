package ecsched

import (
	"os"
	"testing"

	"github.com/k0kubun/pp"
)

func TestLoadConfig(t *testing.T) {
	f, err := os.Open("testdata/sample.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(f)
	if err != nil {
		t.Errorf("error shoud be nil, but: %s", err)
	}
	pp.Println(c)
	r := c.Rules[0]
	pp.Println(r.target())
}
