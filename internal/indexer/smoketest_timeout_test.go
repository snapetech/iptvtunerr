package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snapetech/iptvtunerr/internal/catalog"
)

func TestFilterLiveBySmoketestWithCache_preservesLargeCatalogWhenDurationExpires(t *testing.T) {
	const channelCount = 24908
	var started atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()

	live := make([]catalog.LiveChannel, channelCount)
	for i := range live {
		streamURL := srv.URL + "/stream/" + strconv.Itoa(i)
		live[i] = catalog.LiveChannel{ChannelID: strconv.Itoa(i), StreamURL: streamURL}
	}
	cache := make(SmoketestCache)

	result := FilterLiveBySmoketestWithCache(live, cache, 4*time.Hour, srv.Client(), 8*time.Second, 10, 0, 100*time.Millisecond)

	if got := int(started.Load()); got == 0 {
		t.Fatal("expected the smoketest to start probing channels")
	}
	if len(result) != channelCount {
		t.Fatalf("kept %d/%d channels after the duration limit; want unprobed channels retained", len(result), channelCount)
	}
	if len(cache) != 0 {
		t.Fatalf("cached %d interrupted probes as failures; want no cache entries for probes canceled by the global limit", len(cache))
	}
}

func TestProbeStream_honorsTimeoutWithProvidedClient(t *testing.T) {
	var started atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()

	start := time.Now()
	if ProbeStream(context.Background(), srv.URL+"/slow", srv.Client(), 25*time.Millisecond) {
		t.Fatal("ProbeStream accepted a request that exceeded its per-probe timeout")
	}
	if started.Load() == 0 {
		t.Fatal("expected the probe request to reach the test server")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("ProbeStream returned after %s; expected its 25ms timeout to be enforced", elapsed)
	}
}

func TestFilterLiveBySmoketestWithCache_keepsIncompleteAndUnselectedChannels(t *testing.T) {
	var slowStarted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pass":
			w.Header().Set("Content-Type", "video/mp2t")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0x47, 0x40, 0x00, 0x10})
		case "/fail":
			http.NotFound(w, r)
		case "/slow":
			slowStarted.Add(1)
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	live := []catalog.LiveChannel{
		{ChannelID: "pass", StreamURL: srv.URL + "/pass"},
		{ChannelID: "fail", StreamURL: srv.URL + "/fail"},
		{ChannelID: "slow", StreamURL: srv.URL + "/slow"},
	}
	cache := make(SmoketestCache)
	result, stats := FilterLiveBySmoketestWithCacheReport(live, cache, time.Hour, srv.Client(), time.Second, 3, 0, 100*time.Millisecond)

	if slowStarted.Load() == 0 {
		t.Fatal("expected the slow channel probe to start")
	}
	if got, want := channelIDs(result), []string{"pass", "slow"}; !equalStrings(got, want) {
		t.Fatalf("kept channel ids = %v, want %v", got, want)
	}
	if stats.Passed != 1 || stats.Failed != 1 || stats.Untested != 1 || stats.Invalid != 0 || stats.Kept != 2 {
		t.Fatalf("stats = %+v, want passed=1 failed=1 untested=1 invalid=0 kept=2", stats)
	}
	if result, ok := cache[srv.URL+"/slow"]; ok {
		t.Fatalf("global-timeout probe was cached as a result: %+v", result)
	}
	if result, ok := cache[srv.URL+"/fail"]; !ok || result.Pass {
		t.Fatalf("completed failure was not cached: %+v, exists=%v", result, ok)
	}
}

func TestFilterLiveBySmoketestWithCache_keepsChannelsOutsideProbeSample(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "video/mp2t")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0x47, 0x40, 0x00, 0x10})
	}))
	defer srv.Close()

	live := []catalog.LiveChannel{
		{ChannelID: "a", StreamURL: srv.URL + "/sample-a"},
		{ChannelID: "b", StreamURL: srv.URL + "/sample-b"},
		{ChannelID: "c", StreamURL: srv.URL + "/sample-c"},
	}
	cache := make(SmoketestCache)
	result, stats := FilterLiveBySmoketestWithCacheReport(live, cache, time.Hour, srv.Client(), time.Second, 1, 1, time.Second)

	if got := len(result); got != len(live) {
		t.Fatalf("kept %d/%d channels after probing a sample; want unselected channels retained", got, len(live))
	}
	if got := int(calls.Load()); got != 1 {
		t.Fatalf("made %d requests, want 1 for maxChannels=1", got)
	}
	if stats.Passed != 1 || stats.Failed != 0 || stats.Untested != 2 || stats.Invalid != 0 || stats.Kept != 3 {
		t.Fatalf("stats = %+v, want passed=1 failed=0 untested=2 invalid=0 kept=3", stats)
	}
}

func TestFilterLiveByFeedSmoketestWithCache_keepsFeedsInterruptedByDurationLimit(t *testing.T) {
	var started atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()

	live := []catalog.LiveChannel{{
		ChannelID:  "fallbacks",
		StreamURL:  srv.URL + "/primary",
		StreamURLs: []string{srv.URL + "/primary", srv.URL + "/backup"},
	}}
	cache := make(SmoketestCache)
	result := FilterLiveByFeedSmoketestWithCache(live, cache, time.Hour, srv.Client(), time.Second, 1, 0, 50*time.Millisecond)

	if started.Load() == 0 {
		t.Fatal("expected a feed probe to start")
	}
	if len(result) != 1 || len(result[0].StreamURLs) != 2 {
		t.Fatalf("result = %#v, want the channel and both feeds preserved", result)
	}
	if len(cache) != 0 {
		t.Fatalf("cached %d interrupted feed probes; want none", len(cache))
	}
}

func channelIDs(live []catalog.LiveChannel) []string {
	ids := make([]string, 0, len(live))
	for _, ch := range live {
		ids = append(ids, ch.ChannelID)
	}
	return ids
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
