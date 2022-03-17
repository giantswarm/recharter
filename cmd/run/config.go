package run

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

func loadConfig(path string) ([]syncConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading configuration from %q: %w", path, err)
	}

	config := []syncConfig{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling YAML from %q: %w", path, err)
	}

	return config, nil
}

type syncConfig struct {
	Chart       string `json:"chart"`
	Versions    string `json:"versions"`
	SrcHelmRepo string `json:"srcHelmRepo"`
	DstCatalog  string `json:"dstCatalog"`
}
