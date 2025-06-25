package dns

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLookupIP(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := LookupIP(ctx, "1.1.1.1:53", "unknown.sia.host")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected %q, got %q", ErrNotFound, err)
		}
	})

	t.Run("host", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		res, err := LookupIP(ctx, "1.1.1.1:53", "nomad.sia.host")
		if err != nil {
			t.Fatal(err)
		} else if len(res) == 0 {
			t.Fatal("expected results")
		}
	})
}
