package nyaa

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SearchResult struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Magnet      string    `json:"magnet"`
	Torrent     string    `json:"torrent"`
	Size        string    `json:"size"`
	SizeBytes   int64     `json:"sizeBytes"`
	Date        time.Time `json:"date"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	Downloads   int       `json:"downloads"`
	Category    string    `json:"category"`
	Group       string    `json:"group"`
	Resolution  string    `json:"resolution"`
	IsBatch     bool      `json:"isBatch"`
	IsTrusted   bool      `json:"isTrusted"`
	Score       int       `json:"score"`
	InfoHash    string    `json:"infoHash"`
}

type SearchOptions struct {
	Query           string   `json:"query"`
	Category        string   `json:"category"`
	Filter          string   `json:"filter"`
	User            string   `json:"user"`
	SortBy          string   `json:"sortBy"`
	PreferredGroups []string `json:"preferredGroups"`
	PreferredRes    string   `json:"preferredResolution"`
}

const (
	NyaaBaseURL          = "https://nyaa.si"
	CategoryAnimeAll     = "1_0"
	CategoryAnimeEnglish = "1_2"
	CategoryAnimeRaw     = "1_4"
)

var (
	groupRegex      = regexp.MustCompile(`^\[([^\]]+)\]`)
	resolutionRegex = regexp.MustCompile(`(2160|1080|720|480)[pi]`)
	batchRegex      = regexp.MustCompile(`(?i)(batch|complete|01[-~]\d{2,3}|s\d+\s*complete|\d{2,3}\s*[-~]\s*\d{2,3})`)
	hashRegex       = regexp.MustCompile(`btih:([a-fA-F0-9]{40})`)
)

// Search performs a page-1 HTML scrape of Nyaa results.
func Search(opts SearchOptions) ([]SearchResult, error) {
	results, _, err := SearchHTMLPage(opts, 1)
	return results, err
}

func scoreResult(r SearchResult, opts SearchOptions) int {
	score := 0

	// Seeders
	if r.Seeders > 100 {
		score += 30
	} else if r.Seeders > 50 {
		score += 25
	} else if r.Seeders > 10 {
		score += 20
	} else if r.Seeders > 0 {
		score += 10
	} else {
		score -= 10
	}

	// Preferred group
	for _, g := range opts.PreferredGroups {
		if strings.EqualFold(r.Group, g) {
			score += 25
			break
		}
	}

	// Preferred resolution
	if opts.PreferredRes != "" && r.Resolution == opts.PreferredRes {
		score += 20
	}

	// Batch bonus
	if r.IsBatch {
		score += 15
	}

	// Trusted bonus
	if r.IsTrusted {
		score += 10
	}

	// Encoding/source quality (10bit, x265, BD/bluray)
	titleLower := strings.ToLower(r.Title)
	if strings.Contains(titleLower, "10bit") || strings.Contains(titleLower, "10-bit") ||
		strings.Contains(titleLower, "x265") || strings.Contains(titleLower, "hevc") ||
		strings.Contains(titleLower, "bluray") || strings.Contains(titleLower, "blu-ray") ||
		strings.Contains(titleLower, "bdrip") ||
		strings.Contains(titleLower, " bd ") || strings.HasPrefix(titleLower, "bd ") ||
		strings.Contains(titleLower, "[bd") || strings.Contains(titleLower, "(bd") {
		score += 5
	}

	// Penalize dual audio
	if strings.Contains(titleLower, "dual audio") || strings.Contains(titleLower, "dual.audio") || strings.Contains(titleLower, "multi") {
		score -= 5
	}

	// Downloads popularity
	if r.Downloads > 10000 {
		score += 15
	} else if r.Downloads > 5000 {
		score += 10
	} else if r.Downloads > 1000 {
		score += 5
	}

	// Small batch bonus (under ~25GB)
	if r.IsBatch && r.SizeBytes > 0 && r.SizeBytes < 25*1024*1024*1024 {
		score += 10
	}

	return score
}

func parseSizeToBytes(s string) int64 {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0
	}
	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(parts[1]) {
	case "TIB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GIB":
		return int64(val * 1024 * 1024 * 1024)
	case "MIB":
		return int64(val * 1024 * 1024)
	case "KIB":
		return int64(val * 1024)
	}
	return 0
}
