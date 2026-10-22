package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/samber/lo"
)

const (
	// minDimensionMedia is the width AND height a variant must reach on both axes to
	// be preferred over a cheaper variant.
	minDimensionMedia = 512

	// minDimensionFloor is the width AND height a variant must reach on both axes to
	// be downloaded at all. It exists for panoramas, which have no variant at or
	// above minDimensionMedia on their short axis.
	minDimensionFloor = 256

	// imageContentTypePrefix is the [http.DetectContentType] prefix of every image type.
	imageContentTypePrefix = "image/"
)

// selectPhotoSize returns the cheapest variant that is small enough to download, or
// nil when there is none.
//
// Size comes first: a variant whose FileSize is unknown (0) is treated as eligible,
// because the real limit is enforced again on the downloaded bytes. Only among the
// byte-eligible variants does the preference apply, and it prefers the SMALLEST
// variant reaching minDimensionMedia on both axes, falling back to the smallest one
// still clearing minDimensionFloor on both axes. A variant that clears neither bound
// is still eligible on its own: a narrow panorama has no variant reaching the
// dimension bounds, and it is downloaded as the smallest byte-eligible variant
// instead of being rejected as a download failure.
//
// Filtering by size first is what keeps a 4000x200 panorama - which has no variant
// at or above minDimensionMedia on its short axis - on a narrow variant instead of
// the largest, most expensive one.
//
// The returned pointer aliases the caller's slice, so callers must not mutate it.
func selectPhotoSize(photos []tgbotapi.PhotoSize, maxBytes int64) *tgbotapi.PhotoSize {
	eligible := lo.Filter(photos, func(p tgbotapi.PhotoSize, _ int) bool {
		return p.FileSize == 0 || int64(p.FileSize) <= maxBytes
	})

	if preferred := smallestPhotoSize(lo.Filter(eligible, clearsDimensions(minDimensionMedia))); preferred != nil {
		return preferred
	}

	if floor := smallestPhotoSize(lo.Filter(eligible, clearsDimensions(minDimensionFloor))); floor != nil {
		return floor
	}

	return smallestPhotoSize(eligible)
}

// clearsDimensions builds the predicate selecting the variants whose width AND
// height both reach the given bound.
func clearsDimensions(bound int) func(tgbotapi.PhotoSize, int) bool {
	return func(p tgbotapi.PhotoSize, _ int) bool {
		return p.Width >= bound && p.Height >= bound
	}
}

// smallestPhotoSize returns the variant with the smallest area, or nil for none.
func smallestPhotoSize(photos []tgbotapi.PhotoSize) *tgbotapi.PhotoSize {
	if len(photos) == 0 {
		return nil
	}

	smallest := lo.MinBy(photos, func(a, b tgbotapi.PhotoSize) bool {
		return a.Width*a.Height < b.Width*b.Height
	})

	return &smallest
}
