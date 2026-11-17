package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	args := os.Args[1:]
	exitCode := 0

	for i := range args {
		fmt.Fprintln(os.Stdout, args[i])
		fmt.Fprintln(os.Stderr, args[i])
	}

	if len(args) > 1 {
		num, _ := strconv.Atoi(args[1])
		if args[0] == "exitcode" {
			exitCode = num
		}
		if args[0] == "sleep" {
			time.Sleep(time.Duration(num) * time.Second)
		}
	}
	os.Exit(exitCode)
}
