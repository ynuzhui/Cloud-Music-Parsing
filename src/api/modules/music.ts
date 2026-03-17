import http from "../http";

export type ParseResult = {
  provider: string;
  song_id: string;
  quality: string;
  stream_url: string;
  cache_hit: boolean;
  song_name: string;
  artist_name: string;
  album_name: string;
  album_artist: string;
  year: number;
  track_number: number;
  track_total: number;
  disc_number: number;
  cover_url: string;
};

export type SearchSongItem = {
  id: number;
  name: string;
  artists: string[];
  album: string;
  cover_url: string;
};

export type PlaylistTrack = {
  id: number;
  name: string;
  artists: string[];
  album: string;
  cover_url: string;
};

export type PlaylistInfo = {
  id: number;
  name: string;
  tracks: PlaylistTrack[];
};

export function parseMusic(url: string, quality: string) {
  return http.post<never, ParseResult>("/api/music/parse", { url, quality });
}

export function searchSong(keyword: string, limit = 20) {
  return http.post<never, SearchSongItem[]>("/api/music/search", { keyword, limit });
}

export function fetchPlaylist(id: string) {
  return http.post<never, PlaylistInfo>("/api/music/playlist", { id });
}

export type LyricResult = {
  lyric: string;
  tlyric: string;
};

export function fetchLyric(id: string) {
  return http.post<never, LyricResult>("/api/music/lyric", { id });
}

export type CommentItem = {
  id: number;
  content: string;
  time: number;
  liked_count: number;
  reply_count: number;
  user_id: number;
  user_nickname: string;
  user_avatar_url: string;
};

export type CommentPageResult = {
  total: number;
  has_more: boolean;
  cursor: string;
  comments: CommentItem[];
};

export function fetchComments(params: {
  id: string;
  type?: "song" | "mv" | "playlist" | "album" | "dj" | "video";
  page_no?: number;
  page_size?: number;
  sort_type?: 99 | 2 | 3;
  cursor?: string;
}) {
  return http.post<never, CommentPageResult>("/api/music/comment", params);
}

export type RecommendedPlaylistItem = {
  id: number;
  name: string;
  cover_url: string;
  track_count: number;
  play_count: number;
  copywriter: string;
  creator: string;
};

export function fetchRecommendedPlaylists(limit = 30) {
  return http.post<never, RecommendedPlaylistItem[]>("/api/music/recommend/playlist", { limit });
}

export type ToplistItem = {
  id: number;
  name: string;
  cover_url: string;
  update_frequency: string;
  track_count: number;
  play_count: number;
  tracks_preview: string[];
};

export type ToplistDetailResult = {
  id: number;
  name: string;
  cover_url: string;
  tracks: PlaylistTrack[];
};

export function fetchToplist(id?: string) {
  return http.post<never, ToplistItem[] | ToplistDetailResult>("/api/music/toplist", id ? { id } : {});
}

export type ArtistItem = {
  id: number;
  name: string;
  cover_url: string;
  album_size: number;
  music_size: number;
  mv_size: number;
  trans_names: string[];
};

export type ArtistListResult = {
  more: boolean;
  artists: ArtistItem[];
};

export type ArtistDetailResult = {
  id: number;
  name: string;
  cover_url: string;
  brief: string;
  songs: PlaylistTrack[];
};

export function fetchArtist(params?: { id?: string; limit?: number; offset?: number }) {
  return http.post<never, ArtistListResult | ArtistDetailResult>("/api/music/artist", params ?? {});
}

export function getProviders() {
  return http.get<never, { providers: Array<{ id: string; name: string; description: string }> }>("/api/music/providers");
}

type DownloadAssetResult = {
  blob: Blob;
  fileName: string;
};

function parseFileNameFromDisposition(disposition: string | null): string {
  const raw = disposition || "";
  const utf8Match = raw.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]).trim();
    } catch {
      // ignore decode error and fallback
    }
  }
  const plainMatch = raw.match(/filename="([^"]+)"/i) || raw.match(/filename=([^;]+)/i);
  return plainMatch?.[1]?.trim() || "";
}

async function postDownloadAsset(path: string, id: string, fallbackName: string): Promise<DownloadAssetResult> {
  const token = localStorage.getItem("mp_token");
  const resp = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ id }),
  });

  if (!resp.ok) {
    let message = `请求失败（HTTP ${resp.status}）`;
    try {
      const payload = await resp.json();
      if (payload?.msg) message = payload.msg;
    } catch {
      // ignore parse error
    }
    throw new Error(message);
  }

  const blob = await resp.blob();
  const fileName = parseFileNameFromDisposition(resp.headers.get("content-disposition")) || fallbackName;
  return { blob, fileName };
}

export function downloadLyricAsset(id: string) {
  return postDownloadAsset("/api/music/lyric/download", id, "track.lrc");
}

export function downloadCoverAsset(id: string) {
  return postDownloadAsset("/api/music/cover/download", id, "cover.jpg");
}
