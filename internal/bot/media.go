package bot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/samber/lo"
	"go.uber.org/zap"
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

	// The media notes. A note describes the OUTCOME OF THE FETCH only: it says nothing
	// about whether a model ever rendered a verdict on the image.
	mediaNoteFetched     = "fetched"      // The photo was downloaded and hashed
	mediaNoteTooLarge    = "too_large"    // No variant fitted bot.media.max_bytes
	mediaNoteNotImage    = "not_image"    // The downloaded bytes are not an image
	mediaNoteFetchFailed = "fetch_failed" // Telegram did not deliver the file
)

// selectPhotoSize returns the cheapest variant that is small enough to download, or
// nil when there is none.
//
// Size comes first: a variant whose FileSize is unknown (0) is treated as eligible,
// because the real limit is enforced again on the downloaded bytes. Only among the
// byte-eligible variants does the preference apply, and it prefers the SMALLEST
// variant reaching minDimensionMedia on both axes, falling back to the smallest one
// still clearing minDimensionFloor on both axes. Filtering by size first is what
// keeps a 4000x200 panorama - which has no variant at or above minDimensionMedia on
// its short axis - on a narrow variant instead of the largest, most expensive one.
//
// The returned pointer aliases the caller's slice, so callers must not mutate it.
func selectPhotoSize(photos []tgbotapi.PhotoSize, maxBytes int64) *tgbotapi.PhotoSize {
	eligible := lo.Filter(photos, func(p tgbotapi.PhotoSize, _ int) bool {
		return p.FileSize == 0 || int64(p.FileSize) <= maxBytes
	})

	if preferred := smallestPhotoSize(lo.Filter(eligible, clearsDimensions(minDimensionMedia))); preferred != nil {
		return preferred
	}

	return smallestPhotoSize(lo.Filter(eligible, clearsDimensions(minDimensionFloor)))
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

// fetchMedia downloads the photo of a message and returns it as plugin.Media,
// together with the media note describing the outcome.
//
// ABSENCE and FAILURE never overlap. Nothing to download (ErrNoMedia), nothing small
// enough (ErrMediaTooLarge) and bytes that are not an image (ErrUnsupportedMediaType)
// are absences: they report an Err* sentinel, attach no media and leave the message
// to be evaluated as usual. Any other error is a failure to obtain the image, which
// the caller turns into a block. Both outcomes return a nil media, so the two are
// never confused downstream.
//
// The returned error is safe to log: DownloadFile strips the bot token out of every
// failure before it returns it, and neither the note nor the metadata carries it.
func (b *Bot) fetchMedia(
	ctx context.Context,
	bot *tgbotapifx.Bot,
	message *tgbotapi.Message,
) (*plugin.Media, string, error) {
	// The gate is asked first, so a deployment without a media-capable plugin never
	// downloads anything and never reports a media outcome at all.
	if !b.censor.RequiresMedia() {
		return nil, "", ErrNoMedia
	}

	// A message without a photo is an absence too, not an oversized one.
	if len(message.Photo) == 0 {
		return nil, "", ErrNoMedia
	}

	photo := selectPhotoSize(message.Photo, b.config.Media.MaxBytes)
	if photo == nil {
		b.metrics.IncMediaDownload(MetricLabelMediaResultTooLarge)
		b.logger.Warn("no photo variant fits the media limit",
			zap.Int("variants", len(message.Photo)),
			zap.Int64("limit_bytes", b.config.Media.MaxBytes),
		)

		return nil, mediaNoteTooLarge, fmt.Errorf(
			"%w: %d variants, limit %d bytes",
			ErrMediaTooLarge,
			len(message.Photo),
			b.config.Media.MaxBytes,
		)
	}

	// The download gets its own budget: this deadline is one term of the per-message
	// worst case media.timeout + censor.Timeout.
	downloadCtx, cancel := context.WithTimeout(ctx, b.config.Media.Timeout)
	defer cancel()

	data, err := bot.DownloadFile(downloadCtx, photo.FileID, b.config.Media.MaxBytes)
	if err != nil {
		note, fetchErr := b.reportFailedDownload(err)

		return nil, note, fetchErr
	}

	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, imageContentTypePrefix) {
		b.metrics.IncMediaDownload(MetricLabelMediaResultNotImage)
		b.logger.Warn("downloaded media is not an image",
			zap.String("content_type", contentType),
			zap.Int("size_bytes", len(data)),
		)

		return nil, mediaNoteNotImage, fmt.Errorf("%w: detected %s", ErrUnsupportedMediaType, contentType)
	}

	// Hash once here: no plugin ever re-hashes media that already carries a digest.
	sum := sha256.Sum256(data)

	b.metrics.IncMediaDownload(MetricLabelMediaResultOK)
	b.metrics.AddMediaDownloadBytes(len(data))

	return &plugin.Media{
		Bytes:       data,
		ContentType: contentType,
		SizeBytes:   len(data),
		SHA256:      hex.EncodeToString(sum[:]),
	}, mediaNoteFetched, nil
}

// reportFailedDownload records the outcome of a download that did not deliver a
// usable image and returns the note and error evaluateMessage reports. An oversized
// file is an absence, not a failure, so it keeps the too_large outcome.
func (b *Bot) reportFailedDownload(err error) (string, error) {
	if errors.Is(err, tgbotapifx.ErrFileTooLarge) {
		b.metrics.IncMediaDownload(MetricLabelMediaResultTooLarge)
		b.logger.Warn("downloaded file exceeds the media limit",
			zap.Int64("limit_bytes", b.config.Media.MaxBytes),
		)

		return mediaNoteTooLarge, fmt.Errorf("%w: %w", ErrMediaTooLarge, err)
	}

	b.metrics.IncMediaDownload(MetricLabelMediaResultFetchFailed)
	b.logger.Warn("telegram photo download failed", zap.Error(err))

	return mediaNoteFetchFailed, fmt.Errorf("download photo: %w", err)
}

// isMediaAbsence reports whether err means the message simply has no image to
// examine. It is the exhaustive allowlist of the skip outcomes: anything else is a
// failure to obtain the image, which blocks.
func isMediaAbsence(err error) bool {
	return errors.Is(err, ErrNoMedia) ||
		errors.Is(err, ErrMediaTooLarge) ||
		errors.Is(err, ErrUnsupportedMediaType)
}
