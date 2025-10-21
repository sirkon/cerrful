package main

import "context"

type blobStorage interface {
	getRecord(ctx context.Context, key string) ([]byte, error)
}

func validateConfig(cfg any) error {
	return nil
}

type appConfig struct{}