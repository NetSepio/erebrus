package pkg

import (
	"fmt"

	"github.com/NetSepio/erebrus/util/pkg/caddy"
)

func InstallPkgRequirements() {
	fmt.Println("Starting Install Pkg Requirements for Erebrus")
	caddy.InstallCaddy()
}
