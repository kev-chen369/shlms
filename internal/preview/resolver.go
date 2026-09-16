package preview

import (
	"context"

	"github.com/kev-chen369/shlms/internal/linkresolve"
)

type SafeLinkResolver struct{ Fetcher linkresolve.Fetcher }

func (r SafeLinkResolver) Resolve(ctx context.Context, input string) (string, error) {
	result, err := r.Fetcher.Fetch(ctx, input)
	if err != nil {
		return "", err
	}
	return result.FinalURL, nil
}
