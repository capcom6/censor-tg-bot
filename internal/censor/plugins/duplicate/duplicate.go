package duplicate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	minTextLength = 3 // Minimum text length for duplicate detection

	// textHashPrefix namespaces text hashes inside the duplicate storage.
	textHashPrefix = "text:"
	// imageHashPrefix namespaces image hashes inside the duplicate storage.
	imageHashPrefix = "image:"

	// modalityText labels a candidate built from the message text or caption.
	modalityText = "text"
	// modalityImage labels a candidate built from the message media.
	modalityImage = "image"

	// reasonNoImageContent explains a dropped image candidate.
	reasonNoImageContent = "media carries neither a digest nor bytes"
)

var multiSpaceRegex = regexp.MustCompile(`\s+`)

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "duplicate",
		Factory: func(params map[string]any) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config), nil
		},
	}
}

type Plugin struct {
	storage *Storage
	config  Config
}

func New(config Config) plugin.Plugin {
	return &Plugin{
		storage: NewStorage(config.Window),
		config:  config,
	}
}

func (p *Plugin) Name() string {
	return "duplicate"
}

func (p *Plugin) Priority() int {
	const priority = 150
	return priority
}

// Evaluate records one candidate per available modality and blocks when any of them
// exceeds the duplicate limit.
func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	// Get the message text to analyze
	text := p.getMessageText(msg)

	// Build one candidate per modality, so a photo with no caption is still hashed
	candidates, imageSkip, err := p.buildCandidates(msg, text)
	if err != nil {
		return plugin.Result{}, err
	}

	// Nothing to deduplicate: keep the historical reason byte-identical
	if len(candidates) == 0 {
		metadata := map[string]any{"text_length": len(text)}
		if imageSkip != "" {
			metadata["image_skipped"] = imageSkip
		}

		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "message too short for duplicate detection",
			Metadata: metadata,
			Plugin:   p.Name(),
		}, nil
	}

	// Calculate max occurrences (original + allowed duplicates)
	maxOccurrences := p.config.MaxDuplicates + 1

	// Record every candidate so each modality advances independently, then report
	// the first one that exceeded the limit.
	var (
		offending     *candidate
		offendingStat Entry
	)

	for i := range candidates {
		stat := p.storage.Record(msg.ChatID, candidates[i].hash)
		if offending == nil && stat.Count > maxOccurrences {
			offending, offendingStat = &candidates[i], stat
		}
	}

	if offending != nil {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: fmt.Sprintf(
				"duplicate limit exceeded (%d occurrences, max %d allowed in %s)",
				offendingStat.Count,
				maxOccurrences,
				p.config.Window,
			),
			Metadata: map[string]any{
				"count":           offendingStat.Count,
				"max_occurrences": maxOccurrences,
				"window":          p.config.Window.String(),
				"modality":        offending.modality,
				offending.hashKey: offending.hash,
			},
			Plugin: p.Name(),
		}, nil
	}

	// Within limits, skip to let other plugins decide
	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "duplicate limit not exceeded",
		Metadata: skipMetadata(candidates),
		Plugin:   p.Name(),
	}, nil
}

// NeedsMedia reports whether the plugin needs the media of a message.
// It reads the configuration only, because Media is nil at gate time by construction.
func (p *Plugin) NeedsMedia(_ plugin.Message) bool {
	return p.config.ImageEnabled
}

// candidate is one independent deduplication candidate. The modality is namespaced
// into the hash, so a text hash and an image hash can never collide.
type candidate struct {
	modality string // Modality of the candidate, reported through Result.Metadata
	hash     string // Namespaced hash used as the storage key
	hashKey  string // Result.Metadata key carrying the hash
}

// buildCandidates builds the text and the image candidate independently. The second
// result explains a dropped image candidate and is empty when none was dropped.
func (p *Plugin) buildCandidates(msg plugin.Message, text string) ([]candidate, string, error) {
	// maxCandidates is the number of candidates a single message can produce.
	const maxCandidates = 2

	candidates := make([]candidate, 0, maxCandidates)

	if len(text) >= minTextLength {
		messageHash, err := p.generateMessageHash(text)
		if err != nil {
			return nil, "", err
		}

		candidates = append(candidates, candidate{
			modality: modalityText,
			hash:     messageHash,
			hashKey:  "message_hash",
		})
	}

	if !p.config.ImageEnabled || msg.Media == nil {
		return candidates, "", nil
	}

	imageHash := p.generateImageHash(msg.Media)
	if imageHash == "" {
		return candidates, reasonNoImageContent, nil
	}

	return append(candidates, candidate{
		modality: modalityImage,
		hash:     imageHash,
		hashKey:  "image_hash",
	}), "", nil
}

// skipMetadata reports every recorded hash. Modality is unambiguous only for a
// single candidate, so a message carrying both text and media omits it here.
func skipMetadata(candidates []candidate) map[string]any {
	metadata := make(map[string]any, len(candidates))
	for _, c := range candidates {
		metadata[c.hashKey] = c.hash
	}

	if len(candidates) == 1 {
		metadata["modality"] = candidates[0].modality
	}

	return metadata
}

// getMessageText extracts the primary text content from a message.
// Prefers Text over Caption, falling back to Caption if Text is empty.
func (p *Plugin) getMessageText(msg plugin.Message) string {
	text := strings.TrimSpace(msg.Text)
	if text != "" {
		return p.normalizeText(text)
	}

	// Fallback to caption if text is empty
	caption := strings.TrimSpace(msg.Caption)
	if caption != "" {
		return p.normalizeText(caption)
	}
	return ""
}

func (p *Plugin) normalizeText(text string) string {
	// Convert to lowercase for case-insensitive matching
	text = strings.ToLower(text)
	// Collapse multiple whitespace characters to single space
	text = multiSpaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// generateMessageHash creates a namespaced hash for duplicate detection.
func (p *Plugin) generateMessageHash(text string) (string, error) {
	// Generate hash based on text content
	hasher := fnv.New32a()
	if _, err := hasher.Write([]byte(text)); err != nil {
		return "", fmt.Errorf("failed to generate hash: %w", err)
	}
	return textHashPrefix + hex.EncodeToString(hasher.Sum(nil)), nil
}

// generateImageHash creates a namespaced image hash from the message media.
// It prefers the digest computed by the producer and falls back to hashing the
// bytes, so a producer bug cannot silently disable image deduplication. It returns
// an empty string when the media carries neither a digest nor bytes.
func (p *Plugin) generateImageHash(media *plugin.Media) string {
	if media.SHA256 != "" {
		return imageHashPrefix + media.SHA256
	}

	if len(media.Bytes) == 0 {
		return ""
	}

	sum := sha256.Sum256(media.Bytes)

	return imageHashPrefix + hex.EncodeToString(sum[:])
}

// Cleanup performs maintenance tasks for the plugin.
// Should be called periodically to clean up expired entries.
func (p *Plugin) Cleanup(_ context.Context) {
	p.storage.Cleanup()
}
