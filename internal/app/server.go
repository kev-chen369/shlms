// Package app wires verified identity, PostgreSQL and HTTP service dependencies.
package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/auth"
	"github.com/kev-chen369/shlms/internal/conversion"
	"github.com/kev-chen369/shlms/internal/coupon"
	"github.com/kev-chen369/shlms/internal/dashboard"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
	"github.com/kev-chen369/shlms/internal/httpapi"
	"github.com/kev-chen369/shlms/internal/order"
	"github.com/kev-chen369/shlms/internal/preview"
	"github.com/kev-chen369/shlms/internal/promoter"
)

type Config struct {
	PublicKeyPEM     []byte
	Issuer           string
	Audience         string
	AgreementVersion string
	MigrationsDir    string
}

func NewHandler(ctx context.Context, db *sql.DB, config Config) (http.Handler, error) {
	if db == nil || strings.TrimSpace(config.AgreementVersion) == "" || strings.TrimSpace(config.MigrationsDir) == "" {
		return nil, errors.New("database, agreement version and migrations directory are required")
	}
	verifier, err := auth.NewVerifier(config.PublicKeyPEM, config.Issuer, config.Audience)
	if err != nil {
		return nil, err
	}
	if err = db.PingContext(ctx); err != nil {
		return nil, err
	}
	if err = dbmigrate.Verify(ctx, db, config.MigrationsDir); err != nil {
		return nil, err
	}
	repo := promoter.NewPostgresRepository(db)
	d := httpapi.Dependencies{
		Coupons:      coupon.Catalog{DB: db},
		Orders:       order.ReadStore{DB: db},
		Dashboard:    dashboard.ReadStore{DB: db},
		CouponCities: coupon.Catalog{DB: db},
		ClaimReader:  coupon.ClaimService{Catalog: coupon.Catalog{DB: db}, Store: coupon.ClaimStore{DB: db}},
		Users:        verifier, Admins: auth.AdminResolver{Verifier: verifier, Store: auth.PostgresAdminStore{DB: db}},
		Promoter:                   promoter.Service{Repository: repo},
		PromoterApplications:       promoter.ApplicationService{Repository: repo, AgreementVersion: config.AgreementVersion},
		PromoterCurrentApplication: promoter.CurrentApplicationService{Repository: repo},
		PromoterAdmin:              promoter.AdminService{Repository: repo},
		PromoterAdminList:          promoter.AdminListService{Repository: repo},
		Positions:                  promoter.PositionService{Repository: repo},
		ChannelPositions:           promoter.ChannelPositionService{Repository: repo},
		// The route stays fail-closed until an approved resolver and channel quoter are configured.
		Preview:           preview.Service{Eligibility: preview.PostgresEligibility{DB: db}, Store: preview.NewRepository(db)},
		Conversion:        conversion.Service{Eligibility: preview.PostgresEligibility{DB: db}, Previews: preview.NewRepository(db), Requests: conversion.Repository{DB: db}},
		ConversionReader:  conversion.ReadService{Repository: conversion.Repository{DB: db}},
		ShareArtifacts:    conversion.ShareReader{DB: db},
		ShareEvents:       conversion.ShareEventStore{DB: db},
		NativeShareEvents: conversion.Repository{DB: db},
	}
	return httpapi.NewRouterWithDependencies(d), nil
}
