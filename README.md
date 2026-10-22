# YouTube Sidecar

A small Go desktop sidecar that plays public YouTube and YouTube Music links
through the STR station API used by SiriusXM Sidecar. The desktop setup page
manages saved stations and direct-stream or temporary-download mode. It does
not sign in to YouTube or read browser cookies.

## Quick start

Install Go, [yt-dlp](https://github.com/yt-dlp/yt-dlp), and FFmpeg, and make
sure `yt-dlp` and `ffmpeg` are available on `PATH`. Then build and run:

```powershell
go build -o youtube-sidecar.exe ./server
.\youtube-sidecar.exe
```

Prebuilt executables for Linux, Windows, and macOS are attached to GitHub
releases. yt-dlp and FFmpeg are still required separately.

The app opens its local desktop setup page at `http://127.0.0.1:8093/app` and
serves the STR API on port `8093`. If the browser does not open automatically,
paste that address into a browser on the sidecar computer.

Add a YouTube or YouTube Music video URL in the setup page to save it as a
station. No browser sign-in or cookie setup is needed. yt-dlp accesses videos
without account cookies, so private or account-restricted videos cannot be
played this way.

Add a YouTube or YouTube Music video URL to save it as a station. Supported
links include `youtube.com/watch`, `youtu.be`, YouTube Shorts, YouTube live
URLs, and `music.youtube.com`. The video must be available to yt-dlp without
signing in.

## API

The server listens on `0.0.0.0:8093` by default so STR devices can reach it
over the LAN. Set STR's stream-source base URL to the host's LAN address, for
example `http://192.168.1.25:8093`.

- `GET /api/health` reports service status.
- `GET /api/stations` lists saved station IDs and names.
- `GET /api/stations/{id}` returns station details and its default MP3 URL.
- `GET /api/stations/{id}/now-playing` returns the current video title.
- `GET /api/stations/{id}/stream` streams MP3 audio for speaker compatibility.
- `GET /api/stations/{id}/stream?format=video` streams fragmented MP4 with
  audio and video.

Saved station IDs are the station names, URL-encoded in the stream path. If two
stations have the same name, the later one gets a suffix such as ` (2)`. Older
`youtube-...` stream URLs remain accepted. You can also play a YouTube URL that
was never added to the desktop station list by putting the URL after
`/api/stations/` and before `/stream`. URL-encoding the whole URL is the most
reliable way to preserve query strings:

```text
http://192.168.1.25:8093/api/stations/https%3A%2F%2Fyoutu.be%2FVIDEO_ID/stream
http://192.168.1.25:8093/api/stations/https%3A%2F%2Fmusic.youtube.com%2Fwatch%3Fv%3DVIDEO_ID/stream?format=video
```

Readable links work too, including links with YouTube query parameters:

```text
http://192.168.1.25:8093/api/stations/https://youtu.be/VIDEO_ID/stream
http://192.168.1.25:8093/api/stations/https://www.youtube.com/watch?v=VIDEO_ID/stream
```

The same route accepts YouTube `youtu.be` share links and YouTube Music links.
For audio, the endpoint resolves the best available audio format and converts
it to MP3. For video, it resolves a combined video format and transcodes to a
browser-friendly fragmented MP4 stream. Video playback needs more CPU than
audio playback. The `/app` setup page is restricted to the local computer;
the `/api` endpoints are available to LAN clients.

## Playback modes

- **Direct stream** (default): yt-dlp pipes media into FFmpeg; the full video
  is not saved to disk.
- **Temporary download**: yt-dlp downloads the selected media into a temporary
  folder, then FFmpeg streams it; the file is removed after the request ends.

The temporary-download mode waits for the download to finish before playback.
It is not available for ongoing live streams.

## Settings

- `SIDECAR_HOST`: listener address (default `0.0.0.0`).
- `SIDECAR_PORT`: listener port (default `8093`).
- `SIDECAR_BASE_URL`: public base URL returned to STR; otherwise a LAN IPv4
  address is detected.
- `YOUTUBE_SIDECAR_CONFIG`: config file path (default the current user's
  application config directory under `YouTube-Sidecar/config.json`).
- `YT_DLP`: yt-dlp executable path (default `yt-dlp`).
- `FFMPEG`: FFmpeg executable path (default `ffmpeg`).

The config stores the playback mode and saved station URLs. Allow inbound TCP
traffic on the selected sidecar port for STR devices.

## Notes

yt-dlp and FFmpeg are external programs and should be kept up to date. YouTube
can change its playback behavior, and some videos may require additional
account access, age verification, or may not be available through yt-dlp.
