# yt-transcribe

```
Usage: yt-transcript [options] <youtube-url>

Output a transcript of the given youtube video.

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

Create a transcript of a youtube video:

    yt-transcript 'https://www.youtube.com/watch?v=vP4iY1TtS3s'

Create a transcript with thumbnails every 30 seconds (the default):

    yt-transcript -thumbs 'https://www.youtube.com/watch?v=Ac7G7xOG2Ag'

Create a transcript with thumbnails every 10 seconds (the default):

    yt-transcript -thumbs -thumbinterval 10 'https://www.youtube.com/watch?v=X48G7Y0VWW4'

source: https://github.com/llimllib/yt-transcribe
```
