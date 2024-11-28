package main

import (
	"fmt"
	"os"

	"github.com/goplus/setup-goplus/install"
)

func main() {
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
	if err := install.InstallGop(); err != nil {
		panic(err)
	}
}
