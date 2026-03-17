package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	cacheTTLCommentsExtra  = 1 * time.Minute
	cacheTTLRecommendExtra = 10 * time.Minute
	cacheTTLToplistExtra   = 10 * time.Minute
	cacheTTLArtistExtra    = 20 * time.Minute
)

type CommentItem struct {
	ID            int64  `json:"id"`
	Content       string `json:"content"`
	Time          int64  `json:"time"`
	LikedCount    int64  `json:"liked_count"`
	ReplyCount    int64  `json:"reply_count"`
	UserID        int64  `json:"user_id"`
	UserNickname  string `json:"user_nickname"`
	UserAvatarURL string `json:"user_avatar_url"`
}

type CommentPageResult struct {
	Total    int64         `json:"total"`
	HasMore  bool          `json:"has_more"`
	Cursor   string        `json:"cursor"`
	Comments []CommentItem `json:"comments"`
}

type RecommendedPlaylistItem struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	CoverURL   string `json:"cover_url"`
	TrackCount int    `json:"track_count"`
	PlayCount  int64  `json:"play_count"`
	Copywriter string `json:"copywriter"`
	Creator    string `json:"creator"`
}

type ToplistItem struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	CoverURL        string   `json:"cover_url"`
	UpdateFrequency string   `json:"update_frequency"`
	TrackCount      int      `json:"track_count"`
	PlayCount       int64    `json:"play_count"`
	TracksPreview   []string `json:"tracks_preview"`
}

type ToplistDetailResult struct {
	ID       int64           `json:"id"`
	Name     string          `json:"name"`
	CoverURL string          `json:"cover_url"`
	Tracks   []PlaylistTrack `json:"tracks"`
}

type ArtistItem struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	CoverURL   string   `json:"cover_url"`
	AlbumSize  int      `json:"album_size"`
	MusicSize  int      `json:"music_size"`
	MVSize     int      `json:"mv_size"`
	TransNames []string `json:"trans_names"`
}

type ArtistListResult struct {
	More    bool         `json:"more"`
	Artists []ArtistItem `json:"artists"`
}

type ArtistDetailResult struct {
	ID       int64           `json:"id"`
	Name     string          `json:"name"`
	CoverURL string          `json:"cover_url"`
	Brief    string          `json:"brief"`
	Songs    []PlaylistTrack `json:"songs"`
}

type NeteaseQRCodeKeyResult struct {
	Code    int    `json:"code"`
	Unikey  string `json:"unikey"`
	Message string `json:"message"`
}

type NeteaseQRCodeCheckResult struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Cookie    string `json:"cookie"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

func (s *ParseService) fetchNeteaseLinkWithCookieRetry(ctx context.Context, songID, level string) (string, error) {
	link, err := s.fetchNeteaseLink(ctx, songID, level)
	if err == nil {
		return normalizeExternalMediaURL(link), nil
	}
	if ctx.Err() != nil {
		return "", err
	}

	s.InvalidateCookiePool()
	retryLink, retryErr := s.fetchNeteaseLink(ctx, songID, level)
	if retryErr != nil {
		return "", retryErr
	}
	return normalizeExternalMediaURL(retryLink), nil
}

func (s *ParseService) GetNeteaseQRCodeKey(ctx context.Context) (*NeteaseQRCodeKeyResult, error) {
	body, err := s.doEAPIPost(ctx, "/eapi/login/qrcode/unikey", map[string]any{
		"type": 1,
	})
	if err != nil {
		return nil, err
	}

	var result NeteaseQRCodeKeyResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 || strings.TrimSpace(result.Unikey) == "" {
		if strings.TrimSpace(result.Message) == "" {
			result.Message = "获取二维码 key 失败"
		}
		return nil, errors.New(result.Message)
	}
	result.Unikey = strings.TrimSpace(result.Unikey)
	result.Message = strings.TrimSpace(result.Message)
	return &result, nil
}

func (s *ParseService) CheckNeteaseQRCode(ctx context.Context, key string) (*NeteaseQRCodeCheckResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("二维码 key 不能为空")
	}

	body, err := s.doEAPIPost(ctx, "/eapi/login/qrcode/client/login", map[string]any{
		"key":  key,
		"type": 1,
	})
	if err != nil {
		return nil, err
	}

	var result NeteaseQRCodeCheckResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	result.Message = strings.TrimSpace(result.Message)
	result.Nickname = strings.TrimSpace(result.Nickname)
	result.AvatarURL = normalizeExternalMediaURL(strings.TrimSpace(result.AvatarURL))
	result.Cookie = strings.TrimSpace(result.Cookie)
	return &result, nil
}

func (s *ParseService) FetchComments(
	ctx context.Context,
	resourceType string,
	resourceID string,
	pageNo int,
	pageSize int,
	sortType int,
	cursor string,
) (*CommentPageResult, error) {
	resourceID = strings.TrimSpace(resourceID)
	if !isNumericInput(resourceID) {
		return nil, errors.New("resource id is invalid")
	}

	threadPrefix, err := commentThreadPrefix(resourceType)
	if err != nil {
		return nil, err
	}

	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	switch sortType {
	case 2, 3, 99:
	default:
		sortType = 99
	}

	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		switch sortType {
		case 2:
			cursor = fmt.Sprintf("normalHot#%d", (pageNo-1)*pageSize)
		case 3:
			cursor = "0"
		default:
			cursor = strconv.Itoa((pageNo - 1) * pageSize)
		}
	}

	threadID := threadPrefix + resourceID
	cacheKey := fmt.Sprintf("comment:netease:%s:%d:%d:%d:%s", threadID, pageNo, pageSize, sortType, cursor)
	var cached CommentPageResult
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		for i := range cached.Comments {
			cached.Comments[i].UserAvatarURL = normalizeExternalMediaURL(cached.Comments[i].UserAvatarURL)
		}
		return &cached, nil
	}

	payload := map[string]any{
		"threadId":  threadID,
		"pageNo":    pageNo,
		"showInner": true,
		"pageSize":  pageSize,
		"cursor":    cursor,
		"sortType":  sortType,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/v2/resource/comments", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			TotalCount int64  `json:"totalCount"`
			HasMore    bool   `json:"hasMore"`
			Cursor     string `json:"cursor"`
			Comments   []struct {
				CommentID  int64  `json:"commentId"`
				Content    string `json:"content"`
				Time       int64  `json:"time"`
				LikedCount int64  `json:"likedCount"`
				ReplyCount int64  `json:"replyCount"`
				User       struct {
					UserID    int64  `json:"userId"`
					Nickname  string `json:"nickname"`
					AvatarURL string `json:"avatarUrl"`
				} `json:"user"`
			} `json:"comments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("comment fetch failed, code=%d", result.Code)
	}

	items := make([]CommentItem, 0, len(result.Data.Comments))
	for _, row := range result.Data.Comments {
		items = append(items, CommentItem{
			ID:            row.CommentID,
			Content:       strings.TrimSpace(row.Content),
			Time:          row.Time,
			LikedCount:    row.LikedCount,
			ReplyCount:    row.ReplyCount,
			UserID:        row.User.UserID,
			UserNickname:  strings.TrimSpace(row.User.Nickname),
			UserAvatarURL: normalizeExternalMediaURL(row.User.AvatarURL),
		})
	}

	out := &CommentPageResult{
		Total:    result.Data.TotalCount,
		HasMore:  result.Data.HasMore,
		Cursor:   strings.TrimSpace(result.Data.Cursor),
		Comments: items,
	}
	_ = s.setCacheJSON(ctx, cacheKey, out, cacheTTLCommentsExtra)
	return out, nil
}

func (s *ParseService) FetchRecommendedPlaylists(ctx context.Context, limit int) ([]RecommendedPlaylistItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	cacheKey := fmt.Sprintf("recommend:playlist:netease:%d", limit)
	var cached []RecommendedPlaylistItem
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		for i := range cached {
			cached[i].CoverURL = normalizeExternalMediaURL(cached[i].CoverURL)
		}
		return cached, nil
	}

	payload := map[string]any{
		"limit": limit,
		"total": true,
		"n":     1000,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/personalized/playlist", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code   int `json:"code"`
		Result []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			PicURL     string `json:"picUrl"`
			TrackCount int    `json:"trackCount"`
			PlayCount  int64  `json:"playCount"`
			Copywriter string `json:"copywriter"`
			Creator    struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("recommend playlist fetch failed, code=%d", result.Code)
	}

	items := make([]RecommendedPlaylistItem, 0, len(result.Result))
	for _, row := range result.Result {
		items = append(items, RecommendedPlaylistItem{
			ID:         row.ID,
			Name:       strings.TrimSpace(row.Name),
			CoverURL:   normalizeExternalMediaURL(row.PicURL),
			TrackCount: row.TrackCount,
			PlayCount:  row.PlayCount,
			Copywriter: strings.TrimSpace(row.Copywriter),
			Creator:    strings.TrimSpace(row.Creator.Nickname),
		})
	}
	_ = s.setCacheJSON(ctx, cacheKey, items, cacheTTLRecommendExtra)
	return items, nil
}

func (s *ParseService) FetchToplist(ctx context.Context) ([]ToplistItem, error) {
	cacheKey := "toplist:netease:list"
	var cached []ToplistItem
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		for i := range cached {
			cached[i].CoverURL = normalizeExternalMediaURL(cached[i].CoverURL)
		}
		return cached, nil
	}

	body, err := s.doEAPIPost(ctx, "/eapi/toplist/detail", map[string]any{})
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		List []struct {
			ID              int64  `json:"id"`
			Name            string `json:"name"`
			CoverImgURL     string `json:"coverImgUrl"`
			UpdateFrequency string `json:"updateFrequency"`
			TrackCount      int    `json:"trackCount"`
			PlayCount       int64  `json:"playCount"`
			Tracks          []struct {
				First  string `json:"first"`
				Second string `json:"second"`
			} `json:"tracks"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("toplist fetch failed, code=%d", result.Code)
	}

	items := make([]ToplistItem, 0, len(result.List))
	for _, row := range result.List {
		preview := make([]string, 0, len(row.Tracks))
		for _, t := range row.Tracks {
			title := strings.TrimSpace(t.First)
			artist := strings.TrimSpace(t.Second)
			if title == "" {
				continue
			}
			if artist == "" {
				preview = append(preview, title)
				continue
			}
			preview = append(preview, title+" - "+artist)
		}
		items = append(items, ToplistItem{
			ID:              row.ID,
			Name:            strings.TrimSpace(row.Name),
			CoverURL:        normalizeExternalMediaURL(row.CoverImgURL),
			UpdateFrequency: strings.TrimSpace(row.UpdateFrequency),
			TrackCount:      row.TrackCount,
			PlayCount:       row.PlayCount,
			TracksPreview:   preview,
		})
	}
	_ = s.setCacheJSON(ctx, cacheKey, items, cacheTTLToplistExtra)
	return items, nil
}

func (s *ParseService) FetchToplistDetail(ctx context.Context, id string) (*ToplistDetailResult, error) {
	id = strings.TrimSpace(id)
	if !isNumericInput(id) {
		return nil, errors.New("toplist id is invalid")
	}
	cacheKey := "toplist:netease:detail:" + id
	var cached ToplistDetailResult
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		cached.CoverURL = normalizeExternalMediaURL(cached.CoverURL)
		for i := range cached.Tracks {
			cached.Tracks[i].CoverURL = normalizeExternalMediaURL(cached.Tracks[i].CoverURL)
		}
		return &cached, nil
	}

	payload := map[string]any{
		"id": id,
		"n":  "500",
		"s":  "0",
	}
	body, err := s.doEAPIPost(ctx, "/eapi/playlist/v4/detail", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code     int `json:"code"`
		Playlist struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			CoverImgURL string `json:"coverImgUrl"`
			Tracks      []struct {
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
		return nil, fmt.Errorf("toplist detail fetch failed, code=%d", result.Code)
	}

	tracks := make([]PlaylistTrack, 0, len(result.Playlist.Tracks))
	for _, row := range result.Playlist.Tracks {
		artists := make([]string, 0, len(row.Ar))
		for _, artist := range row.Ar {
			name := strings.TrimSpace(artist.Name)
			if name != "" {
				artists = append(artists, name)
			}
		}
		tracks = append(tracks, PlaylistTrack{
			ID:       row.ID,
			Name:     strings.TrimSpace(row.Name),
			Artists:  artists,
			Album:    strings.TrimSpace(row.Al.Name),
			CoverURL: normalizeExternalMediaURL(row.Al.PicURL),
		})
	}

	out := &ToplistDetailResult{
		ID:       result.Playlist.ID,
		Name:     strings.TrimSpace(result.Playlist.Name),
		CoverURL: normalizeExternalMediaURL(result.Playlist.CoverImgURL),
		Tracks:   tracks,
	}
	_ = s.setCacheJSON(ctx, cacheKey, out, cacheTTLToplistExtra)
	return out, nil
}

func (s *ParseService) FetchArtists(ctx context.Context, limit, offset int) (*ArtistListResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	cacheKey := fmt.Sprintf("artist:netease:list:%d:%d", limit, offset)
	var cached ArtistListResult
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		for i := range cached.Artists {
			cached.Artists[i].CoverURL = normalizeExternalMediaURL(cached.Artists[i].CoverURL)
		}
		return &cached, nil
	}

	payload := map[string]any{
		"limit":  limit,
		"offset": offset,
		"total":  true,
	}
	body, err := s.doEAPIPost(ctx, "/eapi/artist/top", payload)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    int  `json:"code"`
		More    bool `json:"more"`
		Artists []struct {
			ID         int64    `json:"id"`
			Name       string   `json:"name"`
			PicURL     string   `json:"picUrl"`
			AlbumSize  int      `json:"albumSize"`
			MusicSize  int      `json:"musicSize"`
			MVSize     int      `json:"mvSize"`
			TransNames []string `json:"transNames"`
		} `json:"artists"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("artist list fetch failed, code=%d", result.Code)
	}

	items := make([]ArtistItem, 0, len(result.Artists))
	for _, row := range result.Artists {
		items = append(items, ArtistItem{
			ID:         row.ID,
			Name:       strings.TrimSpace(row.Name),
			CoverURL:   normalizeExternalMediaURL(row.PicURL),
			AlbumSize:  row.AlbumSize,
			MusicSize:  row.MusicSize,
			MVSize:     row.MVSize,
			TransNames: row.TransNames,
		})
	}
	out := &ArtistListResult{
		More:    result.More,
		Artists: items,
	}
	_ = s.setCacheJSON(ctx, cacheKey, out, cacheTTLArtistExtra)
	return out, nil
}

func (s *ParseService) FetchArtistDetail(ctx context.Context, id string) (*ArtistDetailResult, error) {
	id = strings.TrimSpace(id)
	if !isNumericInput(id) {
		return nil, errors.New("artist id is invalid")
	}
	cacheKey := "artist:netease:detail:" + id
	var cached ArtistDetailResult
	if ok, err := s.getCacheJSON(ctx, cacheKey, &cached); err == nil && ok {
		cached.CoverURL = normalizeExternalMediaURL(cached.CoverURL)
		for i := range cached.Songs {
			cached.Songs[i].CoverURL = normalizeExternalMediaURL(cached.Songs[i].CoverURL)
		}
		return &cached, nil
	}

	body, err := s.doEAPIPost(ctx, "/eapi/v1/artist/"+id, map[string]any{})
	if err != nil {
		return nil, err
	}

	var result struct {
		Code   int `json:"code"`
		Artist struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			PicURL    string `json:"picUrl"`
			BriefDesc string `json:"briefDesc"`
		} `json:"artist"`
		HotSongs []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Ar   []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Al struct {
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
		} `json:"hotSongs"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("artist detail fetch failed, code=%d", result.Code)
	}

	songs := make([]PlaylistTrack, 0, len(result.HotSongs))
	for _, row := range result.HotSongs {
		artists := make([]string, 0, len(row.Ar))
		for _, artist := range row.Ar {
			name := strings.TrimSpace(artist.Name)
			if name != "" {
				artists = append(artists, name)
			}
		}
		songs = append(songs, PlaylistTrack{
			ID:       row.ID,
			Name:     strings.TrimSpace(row.Name),
			Artists:  artists,
			Album:    strings.TrimSpace(row.Al.Name),
			CoverURL: normalizeExternalMediaURL(row.Al.PicURL),
		})
	}

	out := &ArtistDetailResult{
		ID:       result.Artist.ID,
		Name:     strings.TrimSpace(result.Artist.Name),
		CoverURL: normalizeExternalMediaURL(result.Artist.PicURL),
		Brief:    strings.TrimSpace(result.Artist.BriefDesc),
		Songs:    songs,
	}
	_ = s.setCacheJSON(ctx, cacheKey, out, cacheTTLArtistExtra)
	return out, nil
}

func normalizeExternalMediaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") {
		return "https://" + raw[len("http://"):]
	}
	return raw
}

func commentThreadPrefix(rawType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case "", "song", "music", "0":
		return "R_SO_4_", nil
	case "mv", "1":
		return "R_MV_5_", nil
	case "playlist", "2":
		return "A_PL_0_", nil
	case "album", "3":
		return "R_AL_3_", nil
	case "dj", "radio", "4":
		return "A_DJ_1_", nil
	case "video", "5":
		return "R_VI_62_", nil
	case "event", "6":
		return "A_EV_2_", nil
	case "dr", "7":
		return "A_DR_14_", nil
	default:
		return "", errors.New("comment resource type is not supported")
	}
}
