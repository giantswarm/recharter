package run

import (
	"errors"

	flag "github.com/spf13/pflag"
)

var flags = flagsT{}

type flagsT struct {
	Config      string
	SkipCleanup bool
}

func (f *flagsT) Parse() error {
	flag.StringVar(&flags.Config, "config", "", "Path to the configuration file.")
	flag.BoolVar(&f.SkipCleanup, "skip-cleanup", false, "Do not clean up temporary directories. Useful for debugging.")

	flag.Parse()

	if f.Config == "" {
		return errors.New("--config flag is required")
	}

	return nil
}
