package validate

import (
	"github.com/spf13/cobra"
)

func NoOp(_ *cobra.Command, _ []string) error {
	return nil
}
