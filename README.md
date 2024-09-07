# yt-transcribe

```
Usage: yt-transcribe [options] <youtube-url>

Output a transcribe of the given youtube video.

OPTIONS

  -help:          print this message
  -thumbs:        enable thumbnail generation
  -thumbinterval: the interval between thumbnails, in seconds [default 30]
  -v:             print more verbose output

DEPENDENCIES

Assumes you have installed:

- ffmpeg
- mlx_whisper
- yt-dlp
- python

To install all on a mac:

`brew install ffmpeg python yt-dlp && pip install mlx_whisper`

EXAMPLES

Create a transcribe of a youtube video:

    yt-transcribe 'https://www.youtube.com/watch?v=vP4iY1TtS3s'

Create a transcribe with thumbnails every 30 seconds (the default):

    yt-transcribe -thumbs 'https://www.youtube.com/watch?v=Ac7G7xOG2Ag'

Create a transcribe with thumbnails every 10 seconds (the default):

    yt-transcribe -thumbs -thumbinterval 10 'https://www.youtube.com/watch?v=X48G7Y0VWW4'

source: https://github.com/llimllib/yt-transcribe
```
