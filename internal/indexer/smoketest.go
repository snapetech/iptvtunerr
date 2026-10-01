package indexer

import (
	"bufio"
	"context"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/snapetech/iptvtunerr/internal/catalog"
	"github.com/snapetech/iptvtunerr/internal/httpclient"
	"github.com/snapetech/iptvtunerr/internal/safeurl"
)

// FilterLiveBySmoketest probes each channel's primary stream URL and removes
// channels with a completed failing probe. Uses Range for non-HLS (first 4K
// only) and playlist GET for HLS. Channels that are not probed before a sample
// or duration cap are retained because their status is unknown. maxChannels 0 =
// all; else sample up to maxChannels random URLs. maxDuration caps probe time.
// client may be nil.
func FilterLiveBySmoketest(live []catalog.LiveChannel, client *http.Client, timeout time.Duration, concurrency int, maxChannels int, maxDuration time.Duration) []catalog.LiveChannel {
	return FilterLiveBySmoketestWithCache(live, nil, 0, client, timeout, concurrency, maxChannels, maxDuration)
}

// FilterLiveBySmoketestWithCache is like FilterLiveBySmoketest but skips probing channels
// whose primary URL has a fresh entry in cache. Completed probes update the cache;
// sample- or duration-capped channels remain untested in the returned catalog.
// cache may be nil (behaves identically to FilterLiveBySmoketest). cacheTTL 0 means no caching.
func FilterLiveBySmoketestWithCache(live []catalog.LiveChannel, cache SmoketestCache, cacheTTL time.Duration, client *http.Client, timeout time.Duration, concurrency int, maxChannels int, maxDuration time.Duration) []catalog.LiveChannel {
	filtered, _ := FilterLiveBySmoketestWithCacheReport(live, cache, cacheTTL, client, timeout, concurrency, maxChannels, maxDuration)
	return filtered
}

// SmoketestStats summarizes the channel-level results of a smoketest pass.
// Untested channels have no fresh cache result and were not completed before a
// sample or global-duration cap. They remain in the returned catalog.
type SmoketestStats struct {
	Total    int
	Passed   int
	Failed   int
	Untested int
	Invalid  int
	Kept     int
}

// FilterLiveBySmoketestWithCacheReport is like FilterLiveBySmoketestWithCache
// and also reports how many channels passed, failed, were left untested, or
// lacked a probeable HTTP(S) URL.
func FilterLiveBySmoketestWithCacheReport(live []catalog.LiveChannel, cache SmoketestCache, cacheTTL time.Duration, client *http.Client, timeout time.Duration, concurrency int, maxChannels int, maxDuration time.Duration) ([]catalog.LiveChannel, SmoketestStats) {
	if len(live) == 0 {
		return live, SmoketestStats{}
	}
	if cache == nil {
		cache = make(SmoketestCache)
	}

	primaries := make([]string, len(live))
	urls := make([]string, 0, len(live))
	for i, ch := range live {
		primary := ""
		if len(ch.StreamURLs) > 0 {
			primary = strings.TrimSpace(ch.StreamURLs[0])
		} else {
			primary = strings.TrimSpace(ch.StreamURL)
		}
		if primary == "" || !safeurl.IsHTTPOrHTTPS(primary) {
			continue
		}
		primaries[i] = primary
		urls = append(urls, primary)
	}

	results := probeSmoketestURLs(urls, cache, cacheTTL, client, timeout, concurrency, maxChannels, maxDuration, true)
	filtered := make([]catalog.LiveChannel, 0, len(live))
	stats := SmoketestStats{Total: len(live)}
	for i, ch := range live {
		primary := primaries[i]
		if primary == "" {
			stats.Invalid++
			continue
		}
		result, known := results[primary]
		if !known {
			stats.Untested++
			filtered = append(filtered, ch)
			continue
		}
		if !result.pass {
			stats.Failed++
			continue
		}
		stats.Passed++
		filtered = append(filtered, ch)
	}
	stats.Kept = len(filtered)
	return filtered, stats
}

// FilterLiveByFeedSmoketestWithCache probes every stream URL on each channel,
// prunes completed failing feeds, and keeps untested feeds as fallbacks when
// the sample or duration budget stops the pass early.
// Unprobed URLs are kept if the sample or global duration cap prevents them
// from running. A globally interrupted request is not cached as a failure.
func FilterLiveByFeedSmoketestWithCache(live []catalog.LiveChannel, cache SmoketestCache, cacheTTL time.Duration, client *http.Client, timeout time.Duration, concurrency int, maxFeeds int, maxDuration time.Duration) []catalog.LiveChannel {
	if len(live) == 0 {
		return live
	}
	if cache == nil {
		cache = make(SmoketestCache)
	}

	seen := make(map[string]struct{})
	var urls []string
	for _, ch := range live {
		for _, raw := range liveChannelStreamURLs(ch) {
			u := strings.TrimSpace(raw)
			if u == "" || !safeurl.IsHTTPOrHTTPS(u) {
				continue
			}
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			urls = append(urls, u)
		}
	}
	results := probeSmoketestURLs(urls, cache, cacheTTL, client, timeout, concurrency, maxFeeds, maxDuration, false)

	out := make([]catalog.LiveChannel, 0, len(live))
	for _, ch := range live {
		original := liveChannelStreamURLs(ch)
		var kept []string
		for _, raw := range original {
			u := strings.TrimSpace(raw)
			if u == "" || !safeurl.IsHTTPOrHTTPS(u) {
				continue
			}
			r, known := results[u]
			if !known || r.pass {
				kept = append(kept, u)
			}
		}
		if len(kept) == 0 {
			continue
		}
		next := ch
		next.StreamURL = kept[0]
		next.StreamURLs = kept
		out = append(out, next)
	}
	return out
}

type smoketestProbeResult struct {
	pass bool
}

// probeSmoketestURLs probes unique URLs with a bounded worker pool. Results
// canceled by the overall duration cap are omitted from both the returned map
// and persistent cache, so callers can distinguish an unknown URL from a
// confirmed failure.
func probeSmoketestURLs(urls []string, cache SmoketestCache, cacheTTL time.Duration, client *http.Client, timeout time.Duration, concurrency int, maxProbes int, maxDuration time.Duration, randomize bool) map[string]smoketestProbeResult {
	if cache == nil {
		cache = make(SmoketestCache)
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 10
	}
	if maxDuration <= 0 {
		maxDuration = 5 * time.Minute
	}
	if client == nil {
		client = httpclient.WithTimeout(timeout)
	}

	results := make(map[string]smoketestProbeResult, len(urls))
	seen := make(map[string]struct{}, len(urls))
	var pending []string
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" || !safeurl.IsHTTPOrHTTPS(u) {
			continue
		}
		if _, exists := seen[u]; exists {
			continue
		}
		seen[u] = struct{}{}
		if cacheTTL > 0 {
			if pass, fresh := cache.IsFresh(u, cacheTTL); fresh {
				results[u] = smoketestProbeResult{pass: pass}
				continue
			}
		}
		pending = append(pending, u)
	}

	// Randomize every capped pass, including a duration-only cap, so repeated
	// runs with a persistent cache do not keep favoring the start of the list.
	if randomize && len(pending) > 1 {
		rand.Shuffle(len(pending), func(i, j int) {
			pending[i], pending[j] = pending[j], pending[i]
		})
	}
	if maxProbes > 0 && len(pending) > maxProbes {
		pending = pending[:maxProbes]
	}
	if len(pending) == 0 {
		return results
	}

	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()

	jobs := make(chan string)
	workerCount := concurrency
	if workerCount > len(pending) {
		workerCount = len(pending)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				if ctx.Err() != nil {
					continue
				}
				pass := ProbeStream(ctx, u, client, timeout)
				if ctx.Err() != nil {
					continue
				}
				mu.Lock()
				results[u] = smoketestProbeResult{pass: pass}
				cache[u] = smoketestEntry{Pass: pass, At: time.Now()}
				mu.Unlock()
			}
		}()
	}

	for _, u := range pending {
		if ctx.Err() != nil {
			break
		}
		select {
		case jobs <- u:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	return results
}

func liveChannelStreamURLs(ch catalog.LiveChannel) []string {
	if len(ch.StreamURLs) > 0 {
		return ch.StreamURLs
	}
	if strings.TrimSpace(ch.StreamURL) != "" {
		return []string{ch.StreamURL}
	}
	return nil
}

// ProbeStream returns true if the URL responds with a plausible stream or HLS
// playlist. It rejects empty direct streams and obvious provider black/slate
// redirect targets such as /video/black.ts.
func ProbeStream(ctx context.Context, streamURL string, client *http.Client, timeout time.Duration) bool {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "IptvTunerr/1.0")
	isHLS := strings.HasSuffix(strings.ToLower(streamURL), ".m3u8")
	if !isHLS {
		// Non-HLS: request first 4K only to avoid full-stream bandwidth
		req.Header.Set("Range", "bytes=0-4095")
	}
	probeClient := client
	if client == nil {
		probeClient = httpclient.WithTimeout(timeout)
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	// 206 Partial Content for Range, 200 for full or HLS playlist
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return false
	}
	if isObviousPlaceholderStreamURL(resp.Request.URL.String()) {
		return false
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "mpegurl") || strings.Contains(ct, "m3u8") || isHLS {
		// HLS: scan the playlist before accepting it. Some provider slate
		// playlists start with #EXTM3U and later point every segment at black.ts.
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(nil, 64*1024)
		usable := false
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if isObviousPlaceholderStreamURL(line) {
				return false
			}
			if line == "#EXTM3U" || strings.HasPrefix(line, "#EXTINF") {
				usable = true
				continue
			}
			if line != "" && !strings.HasPrefix(line, "#") {
				usable = true
			}
		}
		return usable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return false
	}
	return len(body) > 0
}

func isObviousPlaceholderStreamURL(raw string) bool {
	u := strings.ToLower(strings.TrimSpace(raw))
	if u == "" {
		return false
	}
	return strings.Contains(u, "/black.ts") ||
		strings.HasSuffix(u, "black.ts") ||
		strings.Contains(u, "/blank.ts") ||
		strings.HasSuffix(u, "blank.ts")
}
