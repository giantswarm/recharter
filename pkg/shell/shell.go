package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type Builder struct {
	command string
	options []option
}

func Cmdf(format string, a ...interface{}) *Builder {
	return &Builder{
		command: fmt.Sprintf(format, a...),
	}
}

// WithCaptureStdoutTo option captures to output of the stdout to the provided
// slice.
func (b *Builder) WithCaptureStdoutTo(bs *[]byte) *Builder {
	o := &captureStdoutToOption{bs: bs}
	b.options = append(b.options, o)
	return b
}

// WithDir option sets the current working directory of the command.
func (b *Builder) WithDir(dir string) *Builder {
	o := &dirOption{dir: dir}
	b.options = append(b.options, o)
	return b
}

func (b *Builder) OrDie() {
	fmt.Printf("--> %s\n", b.command)

	cmd := exec.Command("sh", "-c", b.command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	for _, o := range b.options {
		o.PreRun(cmd)
	}
	err := cmd.Run()
	var eerr *exec.ExitError
	if err != nil && errors.As(err, &eerr) {
		os.Exit(eerr.ExitCode())
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error executing command: %s\n", err)
		os.Exit(1)
	}
	for _, o := range b.options {
		o.PostRun(cmd)
	}
}
