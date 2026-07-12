//go:build darwin

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	trigger = "GO\n"
	fail    = "NO"
	success = "OK"
)

var serviceCmd = &cobra.Command{
	Use:    "service",
	Short:  "Service requests",
	Long:   "Service requests",
	PreRun: silentUsage,
	Run:    serviceRun,
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}

func serviceRun(_ *cobra.Command, _ []string) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&fs.ModeSocket == 0 {
		fmt.Fprintln(os.Stderr, "This is an internal command that is not intended for standard terminal usage")
		os.Exit(1)
	}

	os.Exit(service())
}

func service() int {
	defer os.Stdout.Close()
	defer os.Stdin.Close()
	defer os.Stderr.Close()
	if len(os.Args) < 3 {
		fmt.Print(fail)
		return 1
	}
	target := os.Args[2]

	request := make(chan bool)
	go func() {
		buf := make([]byte, 3)
		_, err := io.ReadFull(os.Stdin, buf)
		request <- err == nil && string(buf) == trigger
	}()

	valid := false
	select {
	case valid = <-request:
	case <-time.After(5 * time.Second):
	}

	if !valid {
		fmt.Println(fail)
		return 2
	}

	err := os.Remove(dockerSock)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		err = os.Symlink(target, dockerSock)
	}

	if err != nil {
		fmt.Print(fail)
		return 3
	}

	fmt.Print(success)
	return 0
}
