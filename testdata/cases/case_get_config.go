package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

func getConfig[T any](store blobStorage, app string) (*T, error) {
	data, err := store.getRecord(context.Background(), path.Join("config", app))
	if err != nil {
		fmt.Printf(
			"Failed to retrieve config data from the given storage: %s. Will fallback to local version.\n",
			err,
		)

		data, err = os.ReadFile(filepath.Join("configs", app))
		if err != nil {
			return nil, fmt.Errorf("read config data stored locally: %w", err)
		}
	}

	var cfg T
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config data: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}