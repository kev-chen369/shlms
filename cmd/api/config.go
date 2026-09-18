package main

import (
	"github.com/kev-chen369/shlms/internal/app"
	"github.com/kev-chen369/shlms/internal/material"
	"os"
)

func loadAPIConfig(publicKey []byte) (app.Config, error) {
	bindings, err := material.LoadCatalogBindingsFile(os.Getenv("CATALOG_BINDINGS_FILE"))
	if err != nil {
		return app.Config{}, err
	}
	return app.Config{
		PublicKeyPEM: publicKey, Issuer: os.Getenv("AUTH_ISSUER"), Audience: os.Getenv("AUTH_AUDIENCE"),
		AgreementVersion: os.Getenv("PROMOTER_AGREEMENT_VERSION"), MigrationsDir: "migrations", CatalogBindings: bindings,
	}, nil
}
