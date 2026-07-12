// Command erebrus is a backward-compatible alias for erebrus-node.
package main

import (
	"os"

	"github.com/NetSepio/erebrus/internal/nodeapp"
)

func main() {
	nodeapp.Main(os.Args)
}
