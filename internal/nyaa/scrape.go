package nyaa

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SearchHTMLPage scrapes a single page of Nyaa HTML results.
func SearchHTMLPage(opts SearchOptions, page int) ([]SearchResult, bool, error) {
	if opts.Category == "" {
		opts.Category = CategoryAnimeEnglish
	}

	params := url.Values{
		"q": {opts.Query},
		"s": {"seeders"},
		"o": {"desc"},
		"p": {strconv.Itoa(page)},
	}

	if opts.Filter != "" {
		params.Set("f", opts.Filter)
	} else {
		params.Set("f", "0")
	}

	if opts.User != "" {
		params.Set("c", "0_0")
		params.Set("u", strings.ToLower(opts.User))
	} else {
		params.Set("c", opts.Category)
	}

	searchURL := fmt.Sprintf("%s/?%s", NyaaBaseURL, params.Encode())
	log.Printf("Nyaa HTML scrape URL (page %d): %s", page, searchURL)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		return nil, false, fmt.Errorf("nyaa request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("nyaa returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse nyaa HTML: %w", err)
	}

	var results []SearchResult

	doc.Find("table.torrent-list tbody tr").Each(func(i int, row *goquery.Selection) {
		r := parseHTMLRow(row)
		if r.Title != "" {
			r.Score = scoreResult(r, opts)
			results = append(results, r)
		}
	})

	// Check if there's a next page — look for pagination with a non-disabled next link,
	// or simply check if any pagination link has a page number > current page
	hasNext := false
	doc.Find("ul.pagination li").Each(func(i int, li *goquery.Selection) {
		// Check for rel="next"
		if li.Find("a[rel='next']").Length() > 0 {
			hasNext = true
			return
		}
		// Check for a "»" or "next" link that isn't disabled
		text := strings.TrimSpace(li.Text())
		if (text == "»" || strings.EqualFold(text, "next")) && !li.HasClass("disabled") {
			if li.Find("a").Length() > 0 {
				hasNext = true
			}
		}
	})

	// Fallback: if we got a full page of results (75), assume there might be more
	if !hasNext && len(results) >= 75 {
		hasNext = true
	}

	return results, hasNext, nil
}

func parseHTMLRow(row *goquery.Selection) SearchResult {
	var r SearchResult

	cols := row.Find("td")
	if cols.Length() < 7 {
		return r
	}

	// Column 0: Category (skip)

	// Column 1: Title + links
	nameCol := cols.Eq(1)
	titleLink := nameCol.Find("a").Last() // last <a> in the name cell is the title
	r.Title = strings.TrimSpace(titleLink.Text())
	if href, exists := titleLink.Attr("href"); exists {
		if strings.HasPrefix(href, "/view/") {
			r.Link = NyaaBaseURL + href
		} else {
			r.Link = href
		}
	}

	// Column 2: Torrent/magnet links
	linkCol := cols.Eq(2)
	linkCol.Find("a").Each(func(i int, a *goquery.Selection) {
		href, exists := a.Attr("href")
		if !exists {
			return
		}
		if strings.HasPrefix(href, "magnet:") {
			r.Magnet = href
		} else if strings.HasSuffix(href, ".torrent") {
			if strings.HasPrefix(href, "/") {
				r.Torrent = NyaaBaseURL + href
			} else {
				r.Torrent = href
			}
		}
	})

	// Column 3: Size
	r.Size = strings.TrimSpace(cols.Eq(3).Text())
	r.SizeBytes = parseSizeToBytes(r.Size)

	// Column 4: Date (timestamp attribute or text)
	dateCol := cols.Eq(4)
	if timestamp, exists := dateCol.Attr("data-timestamp"); exists {
		if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
			r.Date = time.Unix(ts, 0)
		}
	} else {
		dateText := strings.TrimSpace(dateCol.Text())
		if t, err := time.Parse("2006-01-02 15:04", dateText); err == nil {
			r.Date = t
		}
	}

	// Column 5: Seeders
	r.Seeders, _ = strconv.Atoi(strings.TrimSpace(cols.Eq(5).Text()))

	// Column 6: Leechers
	r.Leechers, _ = strconv.Atoi(strings.TrimSpace(cols.Eq(6).Text()))

	// Column 7: Downloads (might not exist on all views)
	if cols.Length() > 7 {
		r.Downloads, _ = strconv.Atoi(strings.TrimSpace(cols.Eq(7).Text()))
	}

	// Trusted: check row class
	rowClass, _ := row.Attr("class")
	r.IsTrusted = strings.Contains(rowClass, "success") || strings.Contains(rowClass, "trusted")

	// Extract info hash from magnet
	if r.Magnet != "" {
		if matches := hashRegex.FindStringSubmatch(r.Magnet); len(matches) > 1 {
			r.InfoHash = strings.ToLower(matches[1])
		}
	}

	// Parse group from title
	if matches := groupRegex.FindStringSubmatch(r.Title); len(matches) > 1 {
		r.Group = matches[1]
	}

	// Parse resolution
	if matches := resolutionRegex.FindStringSubmatch(r.Title); len(matches) > 1 {
		r.Resolution = matches[1] + "p"
	}

	// Detect batch
	r.IsBatch = batchRegex.MatchString(r.Title)

	return r
}

// LoadMorePages fetches additional pages of results.
// Returns all results from page 2 onwards up to maxPages.
func LoadMorePages(opts SearchOptions, maxPages int) ([]SearchResult, error) {
	var allResults []SearchResult

	for page := 2; page <= maxPages+1; page++ {
		results, hasNext, err := SearchHTMLPage(opts, page)
		if err != nil {
			log.Printf("Failed to scrape page %d: %v", page, err)
			break
		}

		allResults = append(allResults, results...)

		if !hasNext || len(results) == 0 {
			break
		}
	}

	return allResults, nil
}
