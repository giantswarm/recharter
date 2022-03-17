package run

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/giantswarm/recharter/pkg/shell"
	"sigs.k8s.io/yaml"
)

const (
	localHelmRepositoryName = "tmp-recharter"
	tmpPrefix               = "./tmp-recharter"
)

func Run() (err error, exitCode int) {
	err = flags.Parse()
	if err != nil {
		return err, 1
	}

	if !flags.SkipCleanup {
		shell.Cmdf("rm -rf %s*", tmpPrefix).OrDie()
		defer shell.Cmdf("rm -rf %s*", tmpPrefix).OrDie()
	}

	config, err := loadConfig(flags.Config)
	if err != nil {
		return err, 1
	}

	// Normalize catalog names.
	for i, _ := range config {
		config[i].DstCatalog = strings.TrimSuffix(config[i].DstCatalog, "-catalog") + "-catalog"
	}

	for _, c := range config {
		err, exitCode := sync(c)
		if err != nil {
			return err, exitCode
		}
	}

	return nil, 0
}

func sync(config syncConfig) (err error, exitCode int) {
	// Download the catalog git repo.

	gitURL := "git@github.com:giantswarm/" + config.DstCatalog + ".git"
	shell.Cmdf("git clone --depth=1 %q ./tmp-recharter-catalog", gitURL).OrDie()

	// Parse source and destination index.yaml.

	srcIndexURL := strings.TrimSuffix(config.SrcHelmRepo, "/") + "/" + "index.yaml"
	resp, err := http.Get(srcIndexURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", srcIndexURL, err), 1
	}
	defer resp.Body.Close()

	srcReleases, err := parseIndex(resp.Body, config.SrcHelmRepo, config.Chart, config.Versions)
	if err != nil {
		return fmt.Errorf("parsing source (%s) index: %w", srcIndexURL, err), 1
	}

	dstIndexPath := "./tmp-recharter-catalog/index.yaml"
	file, err := os.Open(dstIndexPath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dstIndexPath, err), 1
	}
	defer file.Close()

	catalogURL := "https://giantswarm.github.io/" + config.DstCatalog

	dstReleases, err := parseIndex(file, catalogURL, config.Chart, "*")
	if err != nil {
		return fmt.Errorf("parsing destination (%s) index: %w", dstIndexPath, err), 1
	}

	// Pull releases which don't exist in the destination repository.

	shell.Cmdf("mkdir ./tmp-recharter-tarballs").OrDie()

	downloaded := false
	for _, srcReleases := range srcReleases {
		found := false
		for _, dstRelease := range dstReleases {
			if dstRelease.Version == srcReleases.Version {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("--> Chart %s@%s found in the %q catalog, skipping\n", config.Chart, srcReleases.Version, config.DstCatalog)
		} else {
			downloaded = true
			res, err := http.Get(srcReleases.URL)
			if err != nil {
				return fmt.Errorf("downloading tarball from %q: %w", srcReleases.URL, err), 1
			}
			defer res.Body.Close()

			tarballFile := "./tmp-recharter-tarballs/" + srcReleases.URL[strings.LastIndex(srcReleases.URL, "/")+1:]
			file, err := os.OpenFile(tarballFile, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("creating file %q: %w", tarballFile, err), 1
			}
			defer file.Close()

			_, err = io.Copy(file, res.Body)
			if err != nil {
				return fmt.Errorf("copying response content to %q file: %w", tarballFile, err), 1
			}

			shell.Cmdf("helm pull %q --version=%q", config.Chart, srcReleases.Version).
				WithDir("./tmp-recharter-tarballs").
				OrDie()
		}
	}

	// If nothing was downloaded return early.

	if !downloaded {
		fmt.Printf("--> No new releases downloaded from %s\n", config.SrcHelmRepo)
		return nil, 0
	}

	// Update the destination catalog index.yaml and move the downloaded tarballs.

	shell.Cmdf("helm repo index --url=%q --merge=./tmp-recharter-catalog/index.yaml ./tmp-recharter-tarballs", catalogURL).OrDie()
	shell.Cmdf("cp -a ./tmp-recharter-tarballs/* ./tmp-recharter-catalog").OrDie()

	// Commit and push the destination catalog.

	shell.Cmdf("git -C ./tmp-recharter-catalog add -A").OrDie()
	shell.Cmdf("git -C ./tmp-recharter-catalog commit -m %q", "Pull "+config.SrcHelmRepo+" releases").OrDie()
	shell.Cmdf("git -C ./tmp-recharter-catalog push").OrDie()

	return nil, 0
}

func parseIndex(data io.Reader, repoURL, chart, versionRange string) ([]releaseInfo, error) {
	repoURL = strings.TrimSuffix(repoURL, "/index.yaml")
	repoURL = strings.TrimSuffix(repoURL, "/")

	versionConstraint, err := semver.NewConstraint(versionRange)
	if err != nil {
		return nil, fmt.Errorf("creating version constraint for %q: %w", versionRange, err)
	}

	index := struct {
		Entries map[string][]struct {
			Version string   `json:"version"`
			URLs    []string `json:"urls"`
		} `json:"entries"`
	}{}

	bytes, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("reading index bytes: %w", err)
	}
	err = yaml.Unmarshal(bytes, &index)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling index YAML: %w", err)
	}

	releases, ok := index.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("no chart %q found in %s/index.yaml", chart, repoURL)
	}

	var infos []releaseInfo
	for _, r := range releases {
		v, err := semver.NewVersion(r.Version)
		if err != nil {
			return nil, fmt.Errorf("parsing semver for %q: %w", r.Version, err)
		}

		if !versionConstraint.Check(v) {
			continue
		}

		if len(r.URLs) == 0 {
			return nil, fmt.Errorf("no URLs found for %s@%s", chart, r.Version)
		}
		url := r.URLs[0]
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = repoURL + "/" + url
		}
		info := releaseInfo{
			Version: r.Version,
			URL:     url,
		}
		infos = append(infos, info)
	}

	return infos, nil
}
