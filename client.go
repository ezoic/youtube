package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"

	"log/slog"
)

const (
	Size1Kb  = 1024
	Size1Mb  = Size1Kb * 1024
	Size10Mb = Size1Mb * 10

	playerParams = "CgIQBg=="
)

var (
	ErrNoFormat = errors.New("no video format provided")
)

// DefaultClient type to use. No reason to change but you could if you wanted to.
var DefaultClient = IOS

// Client offers methods to download video metadata and video streams.
type Client struct {
	// HTTPClient can be used to set a custom HTTP client.
	// If not set, http.DefaultClient will be used
	HTTPClient *http.Client

	// MaxRoutines to use when downloading a video.
	MaxRoutines int

	// ChunkSize to use when downloading videos in chunks. Default is Size10Mb.
	ChunkSize int64

	// playerCache caches the JavaScript code of a player response
	playerCache playerCache

	client *clientInfo

	ytConfig YTConfig

	consentID string
}

func (c *Client) assureClient() {
	if c.client == nil {
		c.client = &DefaultClient
	}
}

// GetVideo fetches video metadata
func (c *Client) GetVideo(url string) (*Video, error) {
	return c.GetVideoContext(context.Background(), url)
}

// GetVideoContext fetches video metadata with a context
func (c *Client) GetVideoContext(ctx context.Context, url string) (*Video, error) {
	log.Println(url)
	id, err := ExtractVideoID(url)
	if err != nil {
		return nil, fmt.Errorf("extractVideoID failed: %w", err)
	}

	return c.videoFromID(ctx, id)
}

func (c *Client) videoFromID(ctx context.Context, id string) (*Video, error) {
	c.assureClient()

	v := Video{
		ID: id,
	}

	html, err := c.httpGetBodyBytes(ctx, "https://www.youtube.com/watch?v="+id+"&bpctr=9999999999&has_verified=1")
	if err != nil {
		return nil, err
	}

	ytConfig, err := extractYTConfig(html)
	if err != nil {
		return nil, fmt.Errorf("failed to extract yt config: %w", err)
	}

	c.ytConfig = ytConfig

	body, err := c.videoDataByInnertube(ctx, id)
	if err != nil {
		return nil, err
	}

	err = v.parseVideoInfo(body)
	return &v, err
}

type innertubeRequest struct {
	VideoID         string            `json:"videoId,omitempty"`
	BrowseID        string            `json:"browseId,omitempty"`
	Continuation    string            `json:"continuation,omitempty"`
	Context         inntertubeContext `json:"context"`
	PlaybackContext *playbackContext  `json:"playbackContext,omitempty"`
	ContentCheckOK  bool              `json:"contentCheckOk,omitempty"`
	RacyCheckOk     bool              `json:"racyCheckOk,omitempty"`
	Params          string            `json:"params"`
}

type playbackContext struct {
	ContentPlaybackContext contentPlaybackContext `json:"contentPlaybackContext"`
}

type contentPlaybackContext struct {
	SignatureTimestamp string `json:"signatureTimestamp"`
	HTML5Preference    string `json:"html5Preference"`
}

type inntertubeContext struct {
	Client innertubeClient `json:"client"`
}

type innertubeClient struct {
	HL                string `json:"hl"`
	GL                string `json:"gl"`
	ClientName        string `json:"clientName"`
	ClientVersion     string `json:"clientVersion"`
	AndroidSDKVersion int    `json:"androidSDKVersion,omitempty"`
	UserAgent         string `json:"userAgent,omitempty"`
	TimeZone          string `json:"timeZone"`
	UTCOffset         int    `json:"utcOffsetMinutes"`
	DeviceMake        string `json:"deviceMake,omitempty"`
	DeviceModel       string `json:"deviceModel,omitempty"`
	OSName            string `json:"osName,omitempty"`
	OSVersion         string `json:"osVersion,omitempty"`
}

// client info for the innertube API
type clientInfo struct {
	name           string
	key            string
	version        string
	userAgent      string
	androidVersion int
	ytClientName   string
	deviceMake     string
	deviceModel    string
	osName         string
	osVersion      string
}

var (
	IOS = clientInfo{
		name:         "IOS",
		version:      "19.45.4",
		key:          "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
		userAgent:    "com.google.ios.youtube/19.45.4 (iPhone16,2; U; CPU iOS 18_1_0 like Mac OS X;)",
		ytClientName: "5",
		deviceMake:   "Apple",
		deviceModel:  "iPhone16,2",
		osName:       "iPhone",
		osVersion:    "18.1.0.22B83",
	}

	MobileWebClient = clientInfo{
		name:         "MWEB",
		ytClientName: "2",
		version:      "2.20241202.07.00",
		userAgent:    "Mozilla/5.0 (iPad; CPU OS 16_7_10 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1,gzip(gfe)",
	}

	// WebClient, better to use Android client but go ahead.
	WebClient = clientInfo{
		name:      "WEB",
		version:   "2.20220801.00.00",
		key:       "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}

	// AndroidClient, download go brrrrrr.
	AndroidClient = clientInfo{
		name:           "ANDROID",
		version:        "19.44.38",
		key:            "AIzaSyA8eiZmM1FaDVjRy-df2KTyQ_vz_yYM39w",
		userAgent:      "com.google.android.youtube/19.44.38 (Linux; U; Android 11) gzip",
		androidVersion: 30,
		osName:         "Android",
		osVersion:      "11",
	}

	// EmbeddedClient, not really tested.
	EmbeddedClient = clientInfo{
		name:      "WEB_EMBEDDED_PLAYER",
		version:   "1.19700101",
		key:       "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8", // seems like same key works for both clients
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
)

func (c *Client) videoDataByInnertube(ctx context.Context, id string) ([]byte, error) {
	data := innertubeRequest{
		VideoID:        id,
		Context:        prepareInnertubeContext(*c.client),
		ContentCheckOK: true,
		RacyCheckOk:    true,
		//Params:         playerParams,
		PlaybackContext: &playbackContext{
			ContentPlaybackContext: contentPlaybackContext{
				SignatureTimestamp: strconv.Itoa(c.ytConfig.SignatureTimestamp),
				HTML5Preference:    "HTML5_PREF_WANTS",
			},
		},
	}

	return c.httpPostBodyBytes(ctx, "https://www.youtube.com/youtubei/v1/player?prettyPrint=false", data)
}

func (c *Client) transcriptDataByInnertube(ctx context.Context, id string, lang string) ([]byte, error) {
	data := innertubeRequest{
		Context: prepareInnertubeContext(*c.client),
		Params:  transcriptVideoID(id, lang),
	}

	return c.httpPostBodyBytes(ctx, "https://www.youtube.com/youtubei/v1/get_transcript?key="+c.client.key, data)
}

func prepareInnertubeContext(clientInfo clientInfo) inntertubeContext {
	return inntertubeContext{
		Client: innertubeClient{
			HL:                "en",
			GL:                "US",
			TimeZone:          "UTC",
			DeviceMake:        clientInfo.deviceMake,
			DeviceModel:       clientInfo.deviceModel,
			OSName:            clientInfo.osName,
			OSVersion:         clientInfo.osVersion,
			ClientName:        clientInfo.name,
			ClientVersion:     clientInfo.version,
			AndroidSDKVersion: clientInfo.androidVersion,
			UserAgent:         clientInfo.userAgent,
		},
	}
}

func prepareInnertubePlaylistData(ID string, continuation bool, clientInfo clientInfo) innertubeRequest {
	context := prepareInnertubeContext(clientInfo)

	if continuation {
		return innertubeRequest{
			Context:        context,
			Continuation:   ID,
			ContentCheckOK: true,
			RacyCheckOk:    true,
			Params:         playerParams,
		}
	}

	return innertubeRequest{
		Context:        context,
		BrowseID:       "VL" + ID,
		ContentCheckOK: true,
		RacyCheckOk:    true,
		Params:         playerParams,
	}
}

// transcriptVideoID encodes the video ID to the param used to fetch transcripts.
func transcriptVideoID(videoID string, lang string) string {
	langCode := encTranscriptLang(lang)

	// This can be optionally appened to the Sprintf str, not sure what it means
	// *3engagement-panel-searchable-transcript-search-panel\x30\x00\x38\x01\x40\x01
	return base64Enc(fmt.Sprintf("\n\x0b%s\x12\x12%s\x18\x01", videoID, langCode))
}

func encTranscriptLang(languageCode string) string {
	s := fmt.Sprintf("\n\x03asr\x12\x02%s\x1a\x00", languageCode)
	s = base64PadEnc(s)

	return url.QueryEscape(s)
}

// GetPlaylist fetches playlist metadata
func (c *Client) GetPlaylist(url string) (*Playlist, error) {
	return c.GetPlaylistContext(context.Background(), url)
}

// GetPlaylistContext fetches playlist metadata, with a context, along with a list of Videos, and some basic information
// for these videos. Playlist entries cannot be downloaded, as they lack all the required metadata, but
// can be used to enumerate all IDs, Authors, Titles, etc.
func (c *Client) GetPlaylistContext(ctx context.Context, url string) (*Playlist, error) {
	c.assureClient()

	id, err := extractPlaylistID(url)
	if err != nil {
		return nil, fmt.Errorf("extractPlaylistID failed: %w", err)
	}

	data := prepareInnertubePlaylistData(id, false, *c.client)
	body, err := c.httpPostBodyBytes(ctx, "https://www.youtube.com/youtubei/v1/browse?key="+c.client.key, data)
	if err != nil {
		return nil, err
	}

	p := &Playlist{ID: id}
	return p, p.parsePlaylistInfo(ctx, c, body)
}

func (c *Client) VideoFromPlaylistEntry(entry *PlaylistEntry) (*Video, error) {
	return c.videoFromID(context.Background(), entry.ID)
}

func (c *Client) VideoFromPlaylistEntryContext(ctx context.Context, entry *PlaylistEntry) (*Video, error) {
	return c.videoFromID(ctx, entry.ID)
}

// GetStream returns the stream and the total size for a specific format
func (c *Client) GetStream(video *Video, format *Format) (io.ReadCloser, int64, error) {
	return c.GetStreamContext(context.Background(), video, format)
}

// GetStreamContext returns the stream and the total size for a specific format with a context.
func (c *Client) GetStreamContext(ctx context.Context, video *Video, format *Format) (io.ReadCloser, int64, error) {
	url, err := c.GetStreamURL(video, format)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}

	r, w := io.Pipe()
	contentLength := format.ContentLength

	if contentLength == 0 {
		// some videos don't have length information
		contentLength = c.downloadOnce(req, w, format)
	} else {
		// we have length information, let's download by chunks!
		c.downloadChunked(ctx, req, w, format)
	}

	return r, contentLength, nil
}

func (c *Client) downloadOnce(req *http.Request, w *io.PipeWriter, _ *Format) int64 {
	resp, err := c.httpDo(req)
	if err != nil {
		w.CloseWithError(err) //nolint:errcheck
		return 0
	}

	go func() {
		defer resp.Body.Close()
		_, err := io.Copy(w, resp.Body)
		if err == nil {
			w.Close()
		} else {
			w.CloseWithError(err) //nolint:errcheck
		}
	}()

	contentLength := resp.Header.Get("Content-Length")
	length, _ := strconv.ParseInt(contentLength, 10, 64)

	return length
}

func (c *Client) getChunkSize() int64 {
	if c.ChunkSize > 0 {
		return c.ChunkSize
	}

	return Size10Mb
}

func (c *Client) getMaxRoutines(limit int) int {
	routines := 10

	if c.MaxRoutines > 0 {
		routines = c.MaxRoutines
	}

	if limit > 0 && routines > limit {
		routines = limit
	}

	return routines
}

func (c *Client) downloadChunked(ctx context.Context, req *http.Request, w *io.PipeWriter, format *Format) {
	chunks := getChunks(format.ContentLength, c.getChunkSize())
	maxRoutines := c.getMaxRoutines(len(chunks))

	cancelCtx, cancel := context.WithCancel(ctx)
	abort := func(err error) {
		w.CloseWithError(err)
		cancel()
	}

	currentChunk := atomic.Uint32{}
	for i := 0; i < maxRoutines; i++ {
		go func() {
			for {
				chunkIndex := int(currentChunk.Add(1)) - 1
				if chunkIndex >= len(chunks) {
					// no more chunks
					return
				}

				chunk := &chunks[chunkIndex]
				err := c.downloadChunk(req.Clone(cancelCtx), chunk)
				close(chunk.data)

				if err != nil {
					abort(err)
					return
				}
			}
		}()
	}

	go func() {
		// copy chunks into the PipeWriter
		for i := 0; i < len(chunks); i++ {
			select {
			case <-cancelCtx.Done():
				abort(context.Canceled)
				return
			case data := <-chunks[i].data:
				_, err := io.Copy(w, bytes.NewBuffer(data))

				if err != nil {
					abort(err)
				}
			}
		}

		// everything succeeded
		w.Close()
	}()
}

// GetStreamURL returns the url for a specific format
func (c *Client) GetStreamURL(video *Video, format *Format) (string, error) {
	return c.GetStreamURLContext(context.Background(), video, format)
}

// GetStreamURLContext returns the url for a specific format with a context
func (c *Client) GetStreamURLContext(ctx context.Context, video *Video, format *Format) (string, error) {
	if format == nil {
		return "", ErrNoFormat
	}

	c.assureClient()

	if format.URL != "" {
		fmtURL, err := url.Parse(format.URL)
		if err != nil {
			return "", fmt.Errorf("unable to parse url=%s. err=%s", format.URL, err)
		}
		if fmtURL.Query().Get("n") != "" {
			return c.unThrottle(ctx, video.ID, format.URL)
		}

		return format.URL, nil
	}

	// TODO: check rest of this function, is it redundant?

	cipher := format.Cipher
	if cipher == "" {
		return "", ErrCipherNotFound
	}

	uri, err := c.decipherURL(ctx, video.ID, cipher)
	if err != nil {
		return "", err
	}

	return uri, err
}

// httpDo sends an HTTP request and returns an HTTP response.
func (c *Client) httpDo(req *http.Request) (*http.Response, error) {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req.Header.Set("User-Agent", c.client.userAgent)
	req.Header.Set("Origin", "https://youtube.com")
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	if len(c.consentID) == 0 {
		c.consentID = strconv.Itoa(rand.Intn(899) + 100) //nolint:gosec
	}

	req.AddCookie(&http.Cookie{
		Name:   "CONSENT",
		Value:  "YES+cb.20210328-17-p0.en+FX+" + c.consentID,
		Path:   "/",
		Domain: ".youtube.com",
	})

	res, err := client.Do(req)

	log := slog.With("method", req.Method, "url", req.URL)

	if err != nil {
		log.Debug("HTTP request failed", "error", err)
	} else {
		log.Debug("HTTP request succeeded", "status", res.Status)
	}

	return res, err
}

// httpGet does a HTTP GET request, checks the response to be a 200 OK and returns it
func (c *Client) httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code (GET): %d, %s", resp.StatusCode, getErrorBody(body))
	}

	return resp, nil
}

// httpGetBodyBytes reads the whole HTTP body and returns it
func (c *Client) httpGetBodyBytes(ctx context.Context, url string) ([]byte, error) {
	resp, err := c.httpGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// httpPost does a HTTP POST request with a body, checks the response to be a 200 OK and returns it
func (c *Client) httpPost(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Youtube-Client-Name", c.client.ytClientName)
	req.Header.Set("X-Youtube-Client-Version", c.client.version)
	req.Header.Set("User-Agent", c.client.userAgent)
	req.Header.Set("Content-Type", "application/json")
	//req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	// req.Header.Set("X-Goog-Visitor-Id", "CgtzbXhmc3dkdlhVZyjIiIW6BjIKCgJVUxIEGgAgEg=='")

	log.Printf("data: %s. headers: %s", data, req.Header)
	resp, err := c.httpDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code (POST): %d, %s", resp.StatusCode, getErrorBody(body))
	}

	return resp, nil
}

// httpPostBodyBytes reads the whole HTTP body and returns it
func (c *Client) httpPostBodyBytes(ctx context.Context, url string, body interface{}) ([]byte, error) {
	resp, err := c.httpPost(ctx, url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// downloadChunk writes the response data into the data channel of the chunk.
// Downloading in multiple chunks is much faster:
// https://github.com/kkdai/youtube/pull/190
func (c *Client) downloadChunk(req *http.Request, chunk *chunk) error {
	q := req.URL.Query()
	q.Set("range", fmt.Sprintf("%d-%d", chunk.start, chunk.end))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpDo(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= 300 {
		return ErrUnexpectedStatusCode(resp.StatusCode)
	}

	expected := int(chunk.end-chunk.start) + 1
	data, err := io.ReadAll(resp.Body)
	n := len(data)

	if err != nil {
		return err
	}

	if n != expected {
		return fmt.Errorf("chunk at offset %d has invalid size: expected=%d actual=%d", chunk.start, expected, n)
	}

	chunk.data <- data

	return nil
}

func (c *Client) WithClient(ci clientInfo) *Client {
	c.client = &ci
	return c
}

func getErrorBody(b []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err == nil {
		b, _ = json.Marshal(m)
	}
	s := string(b)
	if len(s) > 256 {
		s = s[:256]
	}
	return s
}
