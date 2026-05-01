package main

import (
	"fmt"

	"HerosTime/loginutil"
)

func main() {
	if err := loginutil.GetServerList(); err != nil {
		panic(err)
	}
	first := true
	for i := 1; i <= 80; i++ {
		code := fmt.Sprintf("h5_%d", i)
		if _, _, err := loginutil.ChooseServer(code); err == nil {
			if !first {
				fmt.Print(",")
			}
			fmt.Print(code)
			first = false
		}
	}
	fmt.Println()
}
