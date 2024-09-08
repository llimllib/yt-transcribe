# yt-transcribe

**Transcribe a youtube video into an easily readable HTML file**

I've put up a [sample transcription here](https://cdn.billmill.org/static/yt-transcribe/definedefine/definedefine.html) if you want to see what the output looks like with thumbnails, and a [sample without thumbnails here](http://cdn.billmill.org/static/yt-transcribe/cumberbatch/index.html) if you want to see what that looks like.

## installation

Right now, I think this only works on a mac. To install all the dependencies, run:

`brew install ffmpeg jq python yt-dlp && pip install mlx_whisper`

Once you have the dependencies installed, copy `yt-transcribe` anywhere on your path. I recommend `/usr/local/bin`. Then make sure it's executable with something like `chmod a+x /usr/local/bin/yt-transcribe`.

If you would like to use this on a non-mac computer, [let me know](https://hachyderm.io/@llimllib/); it could be made to work with [whisper.cpp](https://github.com/ggerganov/whisper.cpp) fairly easily.

**Please report any issues you find!**

## usage

```
Usage: yt-transcribe [options] <youtube-url>

Transcribe a youtube video into an easily readable HTML file

OPTIONS

  -help:          print this message
  -outdir:        the directory to put the output files in. [default /tmp/yttranscribe_cache]
  -outfile:       the name of the output HTML file
  -thumbs:        enable thumbnail generation
  -thumbinterval: the interval between thumbnails, in seconds [default 30]
  -v:             print more verbose output

DEPENDENCIES

Assumes you have installed:

- ffmpeg
- jq
- mlx_whisper
- python
- yt-dlp

To install all on a mac:

`brew install ffmpeg jq python yt-dlp && pip install mlx_whisper`

EXAMPLES

Transcribe a youtube video:

    yt-transcribe 'https://www.youtube.com/watch?v=vP4iY1TtS3s'

Transcribe a video and insert thumbnails every 30 seconds (the default):

    yt-transcribe -thumbs 'https://www.youtube.com/watch?v=Ac7G7xOG2Ag'

Transcribe a video and insert thumbnails every 10 seconds:

    yt-transcribe -thumbs -thumbinterval 10 'https://www.youtube.com/watch?v=X48G7Y0VWW4'

Transcribe a video to the 'look-around-you' directory, with a filename 'water.html':

    yt-transcribe -thumbs -outdir ./look-around-you -outfile water.html 'https://www.youtube.com/watch?v=gaI6kBVyu00'

source: https://github.com/llimllib/yt-transcribe
```

# why mlx_whisper instead of whisper.cpp?

Because it's [a lot faster on my machine](https://notes.billmill.org/link_blog/2024/08/mlx-whisper.html)
