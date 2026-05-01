package main

import (
	"fmt"

	"HerosTime/loginutil"
)

func main() {
	if err := loginutil.GetServerList(); err != nil {
		panic(err)
	}
	printExisting("g", 80)
	printExisting("h", 80)
}

func printExisting(prefix string, max int) {
	first := true
	for i := 1; i <= max; i++ {
		code := fmt.Sprintf("%s%d", prefix, i)
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
