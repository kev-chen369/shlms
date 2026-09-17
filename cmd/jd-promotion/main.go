// Command jd-promotion is an operator-only single-product verification tool.
// It is not an end-user endpoint and does not mark channel positions READY.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kev-chen369/shlms/internal/channel"
	"github.com/kev-chen369/shlms/internal/channel/jd"
)

var errArguments = errors.New("product and numeric JD position are required")

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) error {
	flags := flag.NewFlagSet("jd-promotion", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	product := flags.String("product", "", "JD product SKU or HTTPS item URL")
	position := flags.String("position", "", "approved external JD promotion position ID")
	tracking := flags.String("tracking", "", "internal tracking ID, only with approved subUnionId permission")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *product == "" || *position == "" {
		return errArguments
	}
	if *tracking != "" && getenv("JD_SUBUNION_APPROVED") != "true" {
		return jd.ErrTrackingRequired
	}
	client, err := jd.NewHTTPClient(jd.HTTPConfig{
		AppKey: getenv("JD_APP_KEY"), AppSecret: getenv("JD_APP_SECRET"),
		SiteID: getenv("JD_SITE_ID"), Scene2Approved: getenv("JD_SCENE2_APPROVED") == "true",
		SubUnionApproved: getenv("JD_SUBUNION_APPROVED") == "true",
	})
	if err != nil {
		return err
	}
	adapter := jd.NewAdapter(client, jd.Config{PositionID: *position, SubUnionEnabled: *tracking != ""})
	result, err := adapter.CreatePromotionLink(ctx, channel.PromotionRequest{ExternalProductID: *product, TrackingID: *tracking})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, result.URL)
	return err
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "JD product promotion failed:", err)
		os.Exit(1)
	}
}
