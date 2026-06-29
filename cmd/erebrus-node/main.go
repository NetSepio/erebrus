// Command erebrus-node is the Erebrus v2 VPN node binary.
package main

import (
	"os"

	"github.com/NetSepio/erebrus/internal/nodeapp"
)

func main() {
	nodeapp.Main(os.Args)
}