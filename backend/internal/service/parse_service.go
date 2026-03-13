package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-music-aggregator/backend/internal/cache"
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/util"

	"gorm.io/gorm"
)

const (
	cacheTTLParseLink    = 2 * time.Minute
	cacheTTLSongDetail   = 20 * time.Minute
	cacheTTLSearch       = 20 * time.Minute
	cacheTTLPlaylist     = 20 * time.Minute
	cacheTTLLyric        = 6 * time.Hour
	playlistDetailChunk  = 500
	defaultParseQuality  = "standard"
	maxResponseBodySize  = 10 << 20 // 10 MB
	maxCoverBodySize     = 20 << 20 // 20 MB
)

type ParseResult struct {
	Provider    string `json:"provider"`
	SongID      string `json:"song_id"`
	Quality     string `json:"quality"`
	StreamURL   string `json:"stream_url"`
	CacheHit    bool   `json:"cache_hit"`
	SongName    string `json:"song_name"`
	ArtistName  string `json:"artist_name"`
	AlbumName   string `json:"album_name"`
	AlbumArtist string `json:"album_artist"`
	Year        int    `json:"year"`
	TrackNumber int    `json:"track_number"`
	TrackTotal  int    `json:"track_total"`
	DiscNumber  int    `json:"disc_number"`
	CoverURL    string `json:"cover_url"`
}

type SongMeta struct {
	SongName    string `json:"song_name"`
	ArtistName  string `json:"artist_name"`
	AlbumName   string `json:"album_name"`
	AlbumArtist string `json:"album_artist"`
	Year        int    `json:"year"`
	TrackNumber int    `json:"track_number"`
	TrackTotal  int    `json:"track_total"`
	DiscNumber  int    `json:"disc_number"`
	CoverURL    string `json:"cover_url"`
}

type LyricResult struct {
	Lyric  string `json:"lyric"`
	TLyric string `json:"tlyric"`
}

type SearchSongItem struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Artists  []string `json:"artists"`
	Album    string   `json:"album"`
	CoverURL string   `json:"cover_url"`
}

type PlaylistTrack struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Artists  []string `json:"artists"`
	Album    string   `json:"album"`
	CoverURL string   `json:"cover_url"`
}

type PlaylistInfo struct {
	ID     int64           `json:"id"`
	Name   string          `json:"name"`
	Tracks []PlaylistTrack `json:"tracks"`
}

type ParseService struct {
	db             *gorm.DB
	settingService *SettingService
	box            *security.SecretBox
	defaultQuality string
	qualityMu      sync.RWMutex
	cacheMu        sync.RWMutex
	cache          cache.Cache
	httpMu         sync.RWMutex
	httpClient     *http.Client
	httpProxyURL   string // track current proxy config to detect changes
}

func NewParseService(db *gorm.DB, settingSvc *SettingService, box *security.SecretBox) *ParseService {
	return &ParseService{
		db:             db,
		settingService: settingSvc,
		box:            box,
		defaultQuality: defaultParseQuality,
		cache:          cache.NewMemoryCache(),
		httpClient:     &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *ParseService) RefreshCacheBackend(ctx context.Context) error {
	settings, err := s.settingService.Load()
	if err != nil {
		return err
	}
	s.setDefaultQuality(settings.Feature.DefaultParseQuality)
	if !settings.Redis.Enabled {
		s.switchCache(cache.NewMemoryCache())
		return nil
	}
	redisCache := cache.NewRedisCache(settings.Redis.Addr(), settings.Redis.Pass, settings.Redis.DB)
	if err := redisCache.Ping(ctx); err != nil {
		return err
	}
	s.switchCache(redisCache)
	return nil
}

func (s *ParseService) ParseNetease(ctx context.Context, userID uint, requestIP, sourceURL, quality string) (*ParseResult, error) {
	songID, err := extractSongID(sourceURL)
	if err != nil {
		resolvedID, resolveErr := s.resolveSongIDByRedirect(ctx, sourceURL)
		if resolveErr != nil {
			_ = s.recordParse(userID, requestIP, sourceURL, "", quality, false, "failed")
			return nil, fmt.Errorf("%s（重定向解析失败：%v）", err.Error(), resolveErr)
		}
		songID = resolvedID
	}

	level := normalizeQuality(quality, s.getDefaultQuality())
	cacheKey := fmt.Sprintf("parse:netease:%s:%s", songID, level)

	meta, metaErr := s.fetchNeteaseSongDetail(ctx, songID)
	if metaErr != nil {
		log.Printf("[WARN] fetch song detail failed for %s: %v", songID, metaErr)
		meta = &SongMeta{}
	}

	if cached, ok, err := s.getCache(ctx, cacheKey); err == nil && ok {
		result := ParseResult{
			Provider:    "netease",
			SongID:      songID,
			Quality:     level,
			StreamURL:   cached,
			CacheHit:    true,
			SongName:    meta.SongName,
			ArtistName:  meta.ArtistName,
			AlbumName:   meta.AlbumName,
			AlbumArtist: meta.AlbumArtist,
			Year:        meta.Year,
			TrackNumber: meta.TrackNumber,
			TrackTotal:  meta.TrackTotal,
			DiscNumber:  meta.DiscNumber,
			CoverURL:    meta.CoverURL,
		}
		_ = s.recordParse(userID, requestIP, sourceURL, cached, level, true, "success")
		return &result, nil
	}

	link, err := s.fetchNeteaseLink(ctx, songID, level)
	if err != nil {
		_ = s.recordParse(userID, requestIP, sourceURL, "", level, false, "failed")
		return nil, err
	}
	_ = s.setCache(ctx, cacheKey, link, cacheTTLParseLink)
	_ = s.recordParse(userID, requestIP, sourceURL, link, level, false, "success")
	return &ParseResult{
		Provider:    "netease",
		SongID:      songID,
		Quality:     level,
		StreamURL:   link,
		CacheHit:    false,
		SongName:    meta.SongName,
		ArtistName:  meta.ArtistName,
		AlbumName:   meta.AlbumName,
		AlbumArtist: meta.AlbumArtist,
		Year:        meta.Year,
		TrackNumber: meta.TrackNumber,
		TrackTotal:  meta.TrackTotal,
		DiscNumber:  meta.DiscNumber,
		CoverURL:    meta.CoverURL,
	}, nil
}

func (s *ParseService) SearchSong(ctx context.Context, keyword string, limit int) ([]SearchSongItem, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("关键词不能为空")
	}
	if limit <= 0 || limit > 30 {
		limit = 20
	}

	cacheKey := fmt.Sprintf("search:netease:%s:%d", strings.ToLower(keyword), limit)
	var cached []SearchSongItem
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		return cached, nil
	}

	payload := map[string]any{
		"s":      keyword,
		"type":   1,
		"limit":  limit,
		"offset": 0,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/cloudsearch/pc", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code   int `json:"code"`
		Result struct {
			Songs []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"`
				Al struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"al"`
			} `json:"songs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("搜索失败，错误码：%d", result.Code)
	}

	items := make([]SearchSongItem, 0, len(result.Result.Songs))
	for _, song := range result.Result.Songs {
		artists := make([]string, 0, len(song.Ar))
		for _, artist := range song.Ar {
			artists = append(artists, artist.Name)
		}
		items = append(items, SearchSongItem{
			ID:       song.ID,
			Name:     song.Name,
			Artists:  artists,
			Album:    song.Al.Name,
			CoverURL: song.Al.PicURL,
		})
	}
	_ = s.setCacheJSON(ctx, cacheKey, items, cacheTTLSearch)
	return items, nil
}

func (s *ParseService) FetchLyric(ctx context.Context, songID string) (*LyricResult, error) {
	songID = strings.TrimSpace(songID)
	if !digitRegexp.MatchString(songID) {
		return nil, errors.New("歌曲 ID 无效")
	}

	cacheKey := "lyric:netease:" + songID
	var cached LyricResult
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		return &cached, nil
	}

	payload := map[string]any{
		"id":  songID,
		"cp":  false,
		"tv":  0,
		"lv":  0,
		"rv":  0,
		"kv":  0,
		"yv":  0,
		"ytv": 0,
		"yrv": 0,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/song/lyric/v1", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		Lrc  struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
		TLyric struct {
			Lyric string `json:"lyric"`
		} `json:"tlyric"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("歌词获取失败，错误码：%d", result.Code)
	}

	out := &LyricResult{
		Lyric:  result.Lrc.Lyric,
		TLyric: result.TLyric.Lyric,
	}
	_ = s.setCacheJSON(ctx, cacheKey, out, cacheTTLLyric)
	return out, nil
}

func (s *ParseService) BuildLyricDownload(ctx context.Context, songID string) (string, []byte, error) {
	songID = strings.TrimSpace(songID)
	if !isNumericInput(songID) {
		return "", nil, errors.New("歌曲 ID 无效")
	}

	lyric, err := s.FetchLyric(ctx, songID)
	if err != nil {
		return "", nil, err
	}
	content := mergeLyricsForDownload(lyric.Lyric, lyric.TLyric)
	if strings.TrimSpace(content) == "" {
		return "", nil, errors.New("歌词内容为空")
	}

	meta, metaErr := s.fetchNeteaseSongDetail(ctx, songID)
	if metaErr != nil {
		meta = &SongMeta{}
	}
	fileName := buildDownloadFileName(meta.SongName, meta.ArtistName, songID, "lrc")
	return fileName, []byte(content), nil
}

func (s *ParseService) BuildCoverDownload(ctx context.Context, songID string) (string, string, []byte, error) {
	songID = strings.TrimSpace(songID)
	if !isNumericInput(songID) {
		return "", "", nil, errors.New("歌曲 ID 无效")
	}

	meta, err := s.fetchNeteaseSongDetail(ctx, songID)
	if err != nil {
		return "", "", nil, err
	}
	coverURL := strings.TrimSpace(meta.CoverURL)
	if coverURL == "" {
		return "", "", nil, errors.New("封面地址为空")
	}

	client := s.buildHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("User-Agent", util.RandomUserAgent())
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("封面接口返回异常状态码：%d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCoverBodySize))
	if err != nil {
		return "", "", nil, err
	}
	if len(body) == 0 {
		return "", "", nil, errors.New("封面内容为空")
	}

	mime := strings.TrimSpace(resp.Header.Get("Content-Type"))
	ext := detectImageExt(coverURL, mime)
	if mime == "" {
		mime = imageMimeByExt(ext)
	}
	fileName := buildDownloadFileName(meta.SongName, meta.ArtistName, songID, ext)
	return fileName, mime, body, nil
}

func (s *ParseService) FetchPlaylistTracks(ctx context.Context, rawInput string) (*PlaylistInfo, error) {
	playlistID, err := extractPlaylistID(rawInput)
	if err != nil {
		resolvedID, resolveErr := s.resolvePlaylistIDByRedirect(ctx, rawInput)
		if resolveErr != nil {
			return nil, fmt.Errorf("%s（重定向解析失败：%v）", err.Error(), resolveErr)
		}
		playlistID = resolvedID
	}

	cacheKey := "playlist:netease:" + playlistID
	var cached PlaylistInfo
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		return &cached, nil
	}

	detail, err := s.fetchPlaylistDetail(ctx, playlistID)
	if err != nil {
		return nil, err
	}

	orderedIDs := detail.TrackIDs
	if len(orderedIDs) == 0 {
		orderedIDs = make([]int64, 0, len(detail.Tracks))
		for _, track := range detail.Tracks {
			orderedIDs = append(orderedIDs, track.ID)
		}
	}
	orderedIDs = uniqueOrderedIDs(orderedIDs)

	trackMap := make(map[int64]PlaylistTrack, len(detail.Tracks))
	for _, track := range detail.Tracks {
		trackMap[track.ID] = track
	}

	missingIDs := make([]int64, 0)
	for _, id := range orderedIDs {
		if _, ok := trackMap[id]; !ok {
			missingIDs = append(missingIDs, id)
		}
	}

	for _, group := range splitTrackIDs(missingIDs, playlistDetailChunk) {
		tracks, fetchErr := s.fetchSongDetailsByIDs(ctx, group)
		if fetchErr != nil {
			log.Printf("[WARN] fetch playlist detail chunk failed size=%d: %v", len(group), fetchErr)
			continue
		}
		for _, track := range tracks {
			trackMap[track.ID] = track
		}
	}

	finalTracks := make([]PlaylistTrack, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		track, ok := trackMap[id]
		if !ok {
			continue
		}
		finalTracks = append(finalTracks, track)
	}

	if len(finalTracks) == 0 {
		finalTracks = detail.Tracks
	}

	info := &PlaylistInfo{
		ID:     detail.ID,
		Name:   detail.Name,
		Tracks: finalTracks,
	}
	_ = s.setCacheJSON(ctx, cacheKey, info, cacheTTLPlaylist)
	return info, nil
}

func (s *ParseService) fetchNeteaseLink(ctx context.Context, songID, level string) (string, error) {
	requestID := fmt.Sprintf("%d_%04d", time.Now().UnixMilli(), time.Now().Nanosecond()%10000)
	headerConfig := map[string]string{
		"os":        "android",
		"appver":    "9.3.90",
		"osver":     "",
		"deviceId":  "pyncm!",
		"requestId": requestID,
	}
	headerJSON, _ := json.Marshal(headerConfig)

	payload := map[string]any{
		"ids":        []any{json.Number(songID)},
		"level":      level,
		"encodeType": "flac",
		"header":     string(headerJSON),
	}
	if level == "sky" {
		payload["immerseType"] = "c51"
	}
	rawPayload, err := marshalJSONNoEscape(payload)
	if err != nil {
		return "", err
	}

	body, err := s.doEAPIPostRaw(ctx, "/eapi/song/enhance/player/url/v1", rawPayload)
	if err != nil {
		return "", err
	}

	var result struct {
		Code int `json:"code"`
		Data []struct {
			URL   string `json:"url"`
			Code  int    `json:"code"`
			Level string `json:"level"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("网易响应解析失败：%w", err)
	}
	if result.Code != 200 {
		return "", fmt.Errorf("网易接口返回错误码：%d", result.Code)
	}
	if len(result.Data) == 0 {
		return "", errors.New("网易接口返回数据为空")
	}
	if result.Data[0].Code != 200 && result.Data[0].Code != 0 {
		return "", fmt.Errorf("歌曲不可用，错误码：%d（可能需要 VIP 或有效 Cookie）", result.Data[0].Code)
	}
	if strings.TrimSpace(result.Data[0].URL) == "" {
		return "", errors.New("网易返回播放链接为空，请检查 Cookie 是否有效及 VIP 状态")
	}
	return result.Data[0].URL, nil
}

func (s *ParseService) fetchNeteaseSongDetail(ctx context.Context, songID string) (*SongMeta, error) {
	cacheKey := "meta:netease:song:" + songID
	var cached SongMeta
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		return &cached, nil
	}

	payload := map[string]any{
		"c": fmt.Sprintf(`[{"id":%s,"v":0}]`, songID),
	}
	body, err := s.doEAPIPost(ctx, "/eapi/v3/song/detail", payload)
	if err != nil {
		return nil, err
	}

	rows, err := parseSongDetailMetaRows(body)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("未找到歌曲详情")
	}
	first := rows[0]
	meta := &SongMeta{
		SongName:    first.SongName,
		ArtistName:  first.ArtistName,
		AlbumName:   first.AlbumName,
		Year:        first.Year,
		TrackNumber: first.TrackNumber,
		DiscNumber:  first.DiscNumber,
		CoverURL:    first.CoverURL,
	}

	albumMeta, albumErr := s.fetchNeteaseAlbumMeta(ctx, first.AlbumID)
	if albumErr == nil {
		if albumMeta.AlbumArtist != "" {
			meta.AlbumArtist = albumMeta.AlbumArtist
		}
		if albumMeta.Year > 0 {
			meta.Year = albumMeta.Year
		}
		if albumMeta.TrackTotal > 0 {
			meta.TrackTotal = albumMeta.TrackTotal
		}
	}
	if meta.AlbumArtist == "" {
		meta.AlbumArtist = meta.ArtistName
	}
	_ = s.setCacheJSON(ctx, cacheKey, meta, cacheTTLSongDetail)
	return meta, nil
}

type albumMeta struct {
	AlbumArtist string `json:"album_artist"`
	Year        int    `json:"year"`
	TrackTotal  int    `json:"track_total"`
}

func (s *ParseService) fetchNeteaseAlbumMeta(ctx context.Context, albumID int64) (*albumMeta, error) {
	if albumID <= 0 {
		return &albumMeta{}, nil
	}
	cacheKey := fmt.Sprintf("meta:netease:album:%d", albumID)
	var cached albumMeta
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		return &cached, nil
	}

	payload := map[string]any{
		"id": albumID,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/album/v3/detail", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code  int `json:"code"`
		Album struct {
			PublishTime int64 `json:"publishTime"`
			Size        int   `json:"size"`
			Artist      struct {
				Name string `json:"name"`
			} `json:"artist"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"album"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("专辑详情获取失败，错误码：%d", result.Code)
	}

	artistNames := make([]string, 0, len(result.Album.Artists))
	for _, artist := range result.Album.Artists {
		name := strings.TrimSpace(artist.Name)
		if name != "" {
			artistNames = append(artistNames, name)
		}
	}
	albumArtist := strings.Join(artistNames, " / ")
	if albumArtist == "" {
		albumArtist = strings.TrimSpace(result.Album.Artist.Name)
	}

	out := &albumMeta{
		AlbumArtist: albumArtist,
		Year:        yearFromMillis(result.Album.PublishTime),
		TrackTotal:  result.Album.Size,
	}
	_ = s.setCacheJSON(ctx, cacheKey, out, cacheTTLSongDetail)
	return out, nil
}

type songDetailMetaRow struct {
	SongName    string
	ArtistName  string
	AlbumName   string
	AlbumID     int64
	Year        int
	TrackNumber int
	DiscNumber  int
	CoverURL    string
}

func parseSongDetailMetaRows(body []byte) ([]songDetailMetaRow, error) {
	var result struct {
		Code  int `json:"code"`
		Songs []struct {
			Name        string `json:"name"`
			No          int    `json:"no"`
			CD          string `json:"cd"`
			PublishTime int64  `json:"publishTime"`
			Ar          []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Al struct {
				ID     int64  `json:"id"`
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
		} `json:"songs"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("歌曲详情获取失败，错误码：%d", result.Code)
	}

	rows := make([]songDetailMetaRow, 0, len(result.Songs))
	for _, song := range result.Songs {
		artists := make([]string, 0, len(song.Ar))
		for _, artist := range song.Ar {
			name := strings.TrimSpace(artist.Name)
			if name != "" {
				artists = append(artists, name)
			}
		}
		rows = append(rows, songDetailMetaRow{
			SongName:    song.Name,
			ArtistName:  strings.Join(artists, " / "),
			AlbumName:   song.Al.Name,
			AlbumID:     song.Al.ID,
			Year:        yearFromMillis(song.PublishTime),
			TrackNumber: positiveInt(song.No),
			DiscNumber:  parseDiscNumber(song.CD),
			CoverURL:    song.Al.PicURL,
		})
	}
	return rows, nil
}

type playlistDetailPayload struct {
	ID       int64
	Name     string
	TrackIDs []int64
	Tracks   []PlaylistTrack
}

func (s *ParseService) fetchPlaylistDetail(ctx context.Context, playlistID string) (*playlistDetailPayload, error) {
	payload := map[string]any{
		"id": playlistID,
		"n":  1000,
		"s":  0,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/v6/playlist/detail", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code     int `json:"code"`
		Playlist struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			TrackIDs []struct {
				ID int64 `json:"id"`
			} `json:"trackIds"`
			Tracks []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"`
				Al struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"al"`
			} `json:"tracks"`
		} `json:"playlist"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("歌单获取失败，错误码：%d", result.Code)
	}

	trackIDs := make([]int64, 0, len(result.Playlist.TrackIDs))
	for _, row := range result.Playlist.TrackIDs {
		if row.ID > 0 {
			trackIDs = append(trackIDs, row.ID)
		}
	}

	tracks := make([]PlaylistTrack, 0, len(result.Playlist.Tracks))
	for _, song := range result.Playlist.Tracks {
		artists := make([]string, 0, len(song.Ar))
		for _, artist := range song.Ar {
			artists = append(artists, artist.Name)
		}
		tracks = append(tracks, PlaylistTrack{
			ID:       song.ID,
			Name:     song.Name,
			Artists:  artists,
			Album:    song.Al.Name,
			CoverURL: song.Al.PicURL,
		})
	}

	return &playlistDetailPayload{
		ID:       result.Playlist.ID,
		Name:     result.Playlist.Name,
		TrackIDs: trackIDs,
		Tracks:   tracks,
	}, nil
}

func (s *ParseService) fetchSongDetailsByIDs(ctx context.Context, ids []int64) ([]PlaylistTrack, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		items = append(items, map[string]any{"id": id, "v": 0})
	}
	cData, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"c": string(cData),
	}
	body, err := s.doEAPIPost(ctx, "/eapi/v3/song/detail", payload)
	if err != nil {
		return nil, err
	}
	return parseSongDetailTracks(body)
}

func parseSongDetailTracks(body []byte) ([]PlaylistTrack, error) {
	var result struct {
		Code  int `json:"code"`
		Songs []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Ar   []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Al struct {
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
		} `json:"songs"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("歌曲详情获取失败，错误码：%d", result.Code)
	}

	tracks := make([]PlaylistTrack, 0, len(result.Songs))
	for _, song := range result.Songs {
		artists := make([]string, 0, len(song.Ar))
		for _, artist := range song.Ar {
			artists = append(artists, artist.Name)
		}
		tracks = append(tracks, PlaylistTrack{
			ID:       song.ID,
			Name:     song.Name,
			Artists:  artists,
			Album:    song.Al.Name,
			CoverURL: song.Al.PicURL,
		})
	}
	return tracks, nil
}

func (s *ParseService) doEAPIPost(ctx context.Context, apiPath string, payload any) ([]byte, error) {
	rawPayload, err := marshalJSONNoEscape(payload)
	if err != nil {
		return nil, err
	}
	return s.doEAPIPostRaw(ctx, apiPath, rawPayload)
}

func (s *ParseService) doEAPIPostRaw(ctx context.Context, apiPath string, rawPayload string) ([]byte, error) {
	params, err := security.BuildEAPIParams(apiPath, rawPayload)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("params", params)

	client := s.buildHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://interface3.music.163.com"+apiPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	s.setNeteaseHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("网易上游接口状态异常：%d", resp.StatusCode)
	}
	return body, nil
}

func marshalJSONNoEscape(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func (s *ParseService) recordParse(userID uint, requestIP, sourceURL, resultURL, quality string, cacheHit bool, status string) error {
	row := model.ParseRecord{
		UserID:    userID,
		RequestIP: strings.TrimSpace(requestIP),
		Provider:  "netease",
		SourceURL: sourceURL,
		ResultURL: resultURL,
		Quality:   quality,
		CacheHit:  cacheHit,
		Status:    status,
	}
	return s.db.Create(&row).Error
}

func (s *ParseService) getCache(ctx context.Context, key string) (string, bool, error) {
	s.cacheMu.RLock()
	c := s.cache
	s.cacheMu.RUnlock()
	return c.Get(ctx, key)
}

func (s *ParseService) setCache(ctx context.Context, key string, value string, ttl time.Duration) error {
	s.cacheMu.RLock()
	c := s.cache
	s.cacheMu.RUnlock()
	return c.Set(ctx, key, value, ttl)
}

func (s *ParseService) getCacheJSON(ctx context.Context, key string, out any) (bool, error) {
	raw, ok, err := s.getCache(ctx, key)
	if err != nil || !ok {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, err
	}
	return true, nil
}

func (s *ParseService) setCacheJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.setCache(ctx, key, string(raw), ttl)
}

func (s *ParseService) switchCache(next cache.Cache) {
	s.cacheMu.Lock()
	old := s.cache
	s.cache = next
	s.cacheMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
}

func (s *ParseService) setDefaultQuality(raw string) {
	normalized := normalizeQuality(raw, defaultParseQuality)
	s.qualityMu.Lock()
	s.defaultQuality = normalized
	s.qualityMu.Unlock()
}

func (s *ParseService) getDefaultQuality() string {
	s.qualityMu.RLock()
	current := s.defaultQuality
	s.qualityMu.RUnlock()
	if strings.TrimSpace(current) == "" {
		return defaultParseQuality
	}
	return normalizeQuality(current, defaultParseQuality)
}

var digitRegexp = regexp.MustCompile(`\d+`)

func extractSongID(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("歌曲链接不能为空")
	}
	if isNumericInput(rawURL) {
		return rawURL, nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("歌曲链接格式无效")
	}

	if id := extractIDFromQuery(u.Query()); id != "" {
		return id, nil
	}
	if id := extractIDFromFragment(u.Fragment); id != "" {
		return id, nil
	}
	if isNeteaseHost(u.Hostname()) {
		if id := digitRegexp.FindString(u.Path); id != "" {
			return id, nil
		}
	}
	return "", errors.New("未能从链接中提取歌曲 ID")
}

func extractPlaylistID(rawInput string) (string, error) {
	rawInput = strings.TrimSpace(rawInput)
	if rawInput == "" {
		return "", errors.New("歌单 ID 不能为空")
	}
	if isNumericInput(rawInput) {
		return rawInput, nil
	}

	u, err := url.Parse(rawInput)
	if err != nil {
		return "", errors.New("歌单输入格式无效")
	}
	if id := extractIDFromQuery(u.Query()); id != "" {
		return id, nil
	}
	if id := extractIDFromFragment(u.Fragment); id != "" {
		return id, nil
	}
	if isNeteaseHost(u.Hostname()) && strings.Contains(strings.ToLower(u.Path), "playlist") {
		if id := digitRegexp.FindString(u.Path); id != "" {
			return id, nil
		}
	}
	return "", errors.New("未能从输入中提取歌单 ID")
}

func extractIDFromQuery(values url.Values) string {
	id := strings.TrimSpace(values.Get("id"))
	if id == "" {
		return ""
	}
	return digitRegexp.FindString(id)
}

func extractIDFromFragment(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	fragment = strings.TrimPrefix(fragment, "!")

	if idx := strings.Index(fragment, "?"); idx >= 0 && idx+1 < len(fragment) {
		if values, err := url.ParseQuery(fragment[idx+1:]); err == nil {
			if id := extractIDFromQuery(values); id != "" {
				return id
			}
		}
	}
	if values, err := url.ParseQuery(strings.TrimPrefix(fragment, "?")); err == nil {
		if id := extractIDFromQuery(values); id != "" {
			return id
		}
	}
	return ""
}

func isNeteaseHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "music.163.com" {
		return true
	}
	return strings.HasSuffix(host, ".music.163.com")
}

func isNumericInput(raw string) bool {
	return digitRegexp.MatchString(raw) &&
		!strings.Contains(raw, "://") &&
		!strings.Contains(raw, "/") &&
		strings.TrimSpace(raw) == digitRegexp.FindString(raw)
}

func (s *ParseService) resolveSongIDByRedirect(ctx context.Context, rawURL string) (string, error) {
	return s.resolveIDByRedirect(ctx, rawURL, extractSongID)
}

func (s *ParseService) resolvePlaylistIDByRedirect(ctx context.Context, rawURL string) (string, error) {
	return s.resolveIDByRedirect(ctx, rawURL, extractPlaylistID)
}

func (s *ParseService) resolveIDByRedirect(ctx context.Context, rawURL string, extractor func(string) (string, error)) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("链接不能为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("链接格式无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("链接格式无效")
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	client := s.buildHTTPClient()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 8 {
			return errors.New("重定向次数过多")
		}
		return nil
	}

	req, err := http.NewRequestWithContext(resolveCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", util.RandomUserAgent())
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.Request != nil && resp.Request.URL != nil {
		if id, err := extractor(resp.Request.URL.String()); err == nil {
			return id, nil
		}
	}
	if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" && resp.Request != nil && resp.Request.URL != nil {
		if nextURL, err := resp.Request.URL.Parse(location); err == nil {
			if id, err := extractor(nextURL.String()); err == nil {
				return id, nil
			}
		}
	}
	return "", errors.New("重定向后未找到 ID")
}

func normalizeQuality(input, fallback string) string {
	q := strings.ToLower(strings.TrimSpace(input))
	switch q {
	case "standard", "exhigh", "lossless", "hires", "sky", "jyeffect", "jymaster":
		return q
	case "master":
		return "jymaster"
	case "flac":
		return "lossless"
	}

	fb := strings.ToLower(strings.TrimSpace(fallback))
	switch fb {
	case "standard", "exhigh", "lossless", "hires", "sky", "jyeffect", "jymaster":
		return fb
	case "master":
		return "jymaster"
	case "flac":
		return "lossless"
	default:
		return defaultParseQuality
	}
}

func splitTrackIDs(ids []int64, chunkSize int) [][]int64 {
	if chunkSize <= 0 {
		chunkSize = playlistDetailChunk
	}
	if len(ids) == 0 {
		return nil
	}
	chunks := make([][]int64, 0, (len(ids)+chunkSize-1)/chunkSize)
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

func uniqueOrderedIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func yearFromMillis(ms int64) int {
	if ms <= 0 {
		return 0
	}
	return time.UnixMilli(ms).UTC().Year()
}

func parseDiscNumber(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if idx := strings.Index(raw, "/"); idx > 0 {
		raw = raw[:idx]
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func positiveInt(v int) int {
	if v <= 0 {
		return 0
	}
	return v
}

func (s *ParseService) buildHTTPClient() *http.Client {
	settings, _ := s.settingService.Load()
	var wantProxy string
	if settings.Proxy.Enabled {
		wantProxy = strings.TrimSpace(settings.Proxy.BuildURL())
	}

	s.httpMu.RLock()
	if s.httpClient != nil && s.httpProxyURL == wantProxy {
		c := s.httpClient
		s.httpMu.RUnlock()
		return c
	}
	s.httpMu.RUnlock()

	// Rebuild client with updated proxy
	s.httpMu.Lock()
	defer s.httpMu.Unlock()
	// Double-check after acquiring write lock
	if s.httpClient != nil && s.httpProxyURL == wantProxy {
		return s.httpClient
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	if wantProxy != "" {
		if proxyURL, err := url.Parse(wantProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	s.httpClient = &http.Client{Timeout: 20 * time.Second, Transport: transport}
	s.httpProxyURL = wantProxy
	return s.httpClient
}

func (s *ParseService) setNeteaseHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://music.163.com")
	req.Header.Set("User-Agent", util.RandomUserAgent())
	spoofIP := util.RandomIPv4()
	req.Header.Set("X-Forwarded-For", spoofIP)
	req.Header.Set("Client-IP", spoofIP)

	defaultCookies := map[string]string{
		"os":       "android",
		"appver":   "9.3.90",
		"osver":    "",
		"deviceId": "pyncm!",
	}
	for k, v := range defaultCookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	if musicU := s.pickActiveCookie(); musicU != "" {
		req.AddCookie(&http.Cookie{Name: "MUSIC_U", Value: musicU})
	}
}

func mergeLyricsForDownload(lyric, tlyric string) string {
	main := normalizeNeteaseLyricForDownload(lyric)
	trans := normalizeNeteaseLyricForDownload(tlyric)
	if main == "" && trans == "" {
		return ""
	}
	if main == "" {
		return trans
	}
	if trans == "" {
		return main
	}
	return main + "\n\n[Translation]\n" + trans
}

func normalizeNeteaseLyricForDownload(raw string) string {
	source := strings.ReplaceAll(raw, "\r", "")
	if strings.TrimSpace(source) == "" {
		return ""
	}

	lines := strings.Split(source, "\n")
	output := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			output = append(output, "")
			continue
		}
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			var obj struct {
				T int `json:"t"`
				C []struct {
					TX string `json:"tx"`
				} `json:"c"`
			}
			if err := json.Unmarshal([]byte(trimmed), &obj); err == nil && len(obj.C) > 0 {
				var text strings.Builder
				for _, part := range obj.C {
					text.WriteString(part.TX)
				}
				output = append(output, fmt.Sprintf("[%s]%s", formatLrcTimestamp(obj.T), strings.TrimSpace(text.String())))
				continue
			}
		}
		output = append(output, line)
	}

	joined := strings.TrimSpace(strings.Join(output, "\n"))
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return joined
}

func formatLrcTimestamp(ms int) string {
	if ms < 0 {
		ms = 0
	}
	min := ms / 60000
	sec := (ms % 60000) / 1000
	milli := ms % 1000
	return fmt.Sprintf("%02d:%02d.%03d", min, sec, milli)
}

func detectImageExt(rawURL, mime string) string {
	lowerMime := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.Contains(lowerMime, "png"):
		return "png"
	case strings.Contains(lowerMime, "webp"):
		return "webp"
	case strings.Contains(lowerMime, "gif"):
		return "gif"
	case strings.Contains(lowerMime, "bmp"):
		return "bmp"
	case strings.Contains(lowerMime, "jpg"), strings.Contains(lowerMime, "jpeg"):
		return "jpg"
	}

	pureURL := strings.ToLower(strings.TrimSpace(rawURL))
	if idx := strings.Index(pureURL, "?"); idx >= 0 {
		pureURL = pureURL[:idx]
	}
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp"} {
		if strings.HasSuffix(pureURL, ext) {
			if ext == ".jpeg" {
				return "jpg"
			}
			return strings.TrimPrefix(ext, ".")
		}
	}
	return "jpg"
}

func imageMimeByExt(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}

func buildDownloadFileName(songName, artistName, songID, ext string) string {
	title := strings.TrimSpace(songName)
	if title == "" {
		title = "song_" + strings.TrimSpace(songID)
	}
	artist := strings.TrimSpace(artistName)
	base := title
	if artist != "" {
		base = title + " - " + artist
	}
	base = sanitizeDownloadFileName(base)
	if base == "" {
		base = "track"
	}
	cleanExt := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
	if cleanExt == "" {
		cleanExt = "bin"
	}
	return base + "." + cleanExt
}

func sanitizeDownloadFileName(raw string) string {
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\r", "",
		"\n", "",
	)
	return strings.TrimSpace(replacer.Replace(raw))
}
