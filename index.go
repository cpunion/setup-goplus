package main

import "github.com/goplus/setup-goplus/install"

func main() {
	if err := install.InstallGop(); err != nil {
		panic(err)
	}
}
