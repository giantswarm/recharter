package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	flag "github.com/spf13/pflag"
)

const (
	localHelmRepositoryName = "tmp-recharter"
)

var options = optionsT{}

var flags = flagsT{}

func main() {
	err, code := mainE()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	}
	os.Exit(code)
}

func mainE() (err error, exitCode int) {
	err = flags.Parse()
	if err != nil {
		return err, 1
	}

	if !flags.SkipCleanup {
		sh("rm -rf ./tmp-recharter*")
		defer sh("rm -rf ./tmp-recharter*")
	}

	sh(fmt.Sprintf("helm repo add %s https://helm.cilium.io/", localHelmRepositoryName))
	defer sh(fmt.Sprintf("helm repo remove %s", localHelmRepositoryName))

	var releasesJson []byte
	sh(fmt.Sprintf("helm search repo --versions --version=%q -o json %q", flags.ReleaseVersion, localHelmRepositoryName),
		options.CaptureStdoutTo(&releasesJson),
	)

	releases := []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}{}
	err = json.Unmarshal(releasesJson, &releases)
	if err != nil {
		return fmt.Errorf("unmarshaling releases JSON: %w", err), 1
	}

	sh("mkdir ./tmp-recharter-tarballs")
	for _, r := range releases {
		sh(fmt.Sprintf("helm pull %q --version=%q", r.Name, r.Version),
			options.Dir("./tmp-recharter-tarballs"),
		)
	}

	gitURL := "git@github.com:giantswarm/" + flags.Catalog + ".git"
	sh(fmt.Sprintf("git clone --depth=1 %q ./tmp-recharter-catalog", gitURL))

	catalogURL := "https://giantswarm.github.io/" + flags.Catalog
	sh(fmt.Sprintf("helm repo index --url=%q --merge=./tmp-recharter-catalog/index.yaml ./tmp-recharter-tarballs", catalogURL))
	sh("cp -a ./tmp-recharter-tarballs/* ./tmp-recharter-catalog")

	sh("git -C ./tmp-recharter-catalog add -A")
	sh(fmt.Sprintf("git -C ./tmp-recharter-catalog commit -m %q", "Pull "+flags.RepoURL+" releases"))
	sh("git -C ./tmp-recharter-catalog push")

	return nil, 0
}

func sh(command string, opts ...option) {
	fmt.Printf("--> %s\n", command)

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	for _, o := range opts {
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
	for _, o := range opts {
		o.PostRun(cmd)
	}
}

type flagsT struct {
	RepoURL        string
	Catalog        string
	ReleaseVersion string
	SkipCleanup    bool
}

func (f *flagsT) Parse() error {
	flag.StringVar(&flags.RepoURL, "repo-url", "", "Source Helm repository URL.")
	flag.StringVar(&flags.Catalog, "catalog", "", "App Catalog name to push the chart to.")
	flag.StringVar(&flags.ReleaseVersion, "release-version", "", "Filter releases using semantic versioning constraints, e.g. \"^1.0.0\".")
	flag.BoolVar(&f.SkipCleanup, "skip-cleanup", false, "Do not clean up temporary directories. Useful for debugging.")

	flag.Parse()

	if f.RepoURL == "" {
		return errors.New("--repo-url flag is required")
	}
	if f.Catalog == "" {
		return errors.New("--catalog flag is required")
	}

	return nil
}

type option interface {
	PreRun(*exec.Cmd)
	PostRun(*exec.Cmd)
}

type optionsT struct{}

// CaptureStdoutTo captures to output of the stdout to the provided slice.
func (optionsT) CaptureStdoutTo(bs *[]byte) *captureStdoutToOption {
	return &captureStdoutToOption{bs: bs}
}

// Dir sets the current working directory of the command.
func (optionsT) Dir(dir string) *dirOption {
	return &dirOption{dir: dir}
}

type captureStdoutToOption struct {
	bs  *[]byte
	buf *bytes.Buffer
}

func (o *captureStdoutToOption) PreRun(cmd *exec.Cmd) {
	o.buf = new(bytes.Buffer)
	cmd.Stdout = o.buf
}

func (o *captureStdoutToOption) PostRun(cmd *exec.Cmd) {
	*o.bs = o.buf.Bytes()
}

type dirOption struct {
	dir string
}

func (o *dirOption) PreRun(cmd *exec.Cmd) {
	cmd.Dir = o.dir
}

func (o *dirOption) PostRun(cmd *exec.Cmd) {
}
