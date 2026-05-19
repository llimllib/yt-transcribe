# yt-transcribe

**Transcribe a youtube video into an easily readable HTML file**

I've put up a [sample transcription here](https://llimllib.github.io/yt-transcribe/definedefine/definedefine.html) if you want to see what the output looks like with thumbnails, and a [sample without thumbnails here](https://llimllib.github.io/yt-transcribe/cumberbatch/) if you want to see what that looks like.

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
  -model:         specify the whisper model to use [default mlx-community/distil-whisper-large-v3]
  -outdir:        the directory to put the output files in. [default /tmp/yttranscribe_cache]
  -outfile:       the name of the output HTML file
  -thumbs:        enable thumbnail generation
  -thumbinterval: the interval between thumbnails, in seconds [default 30]
  -thumbwidth:    the width of the thumbnails, defaults to 640
  -adaptive:      use scene detection to adaptively choose thumbnail times
  -scenethresh:   scene detection threshold (0.0-1.0) [default 0.03]
  -mingap:        minimum seconds between adaptive thumbnails [default 10]
  -maxgap:        maximum seconds without a thumbnail (fallback interval) [default 60]
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

Transcribe with adaptive scene-detection thumbnails:

    yt-transcribe -thumbs -adaptive 'https://www.youtube.com/watch?v=X48G7Y0VWW4'

Adaptive with custom thresholds (more sensitive, tighter gaps):

    yt-transcribe -thumbs -adaptive -scenethresh 0.02 -mingap 5 -maxgap 45 'https://www.youtube.com/watch?v=X48G7Y0VWW4'

Transcribe a video to the 'look-around-you' directory, with a filename 'water.html':

    yt-transcribe -thumbs -outdir ./look-around-you -outfile water.html 'https://www.youtube.com/watch?v=gaI6kBVyu00'

source: https://github.com/llimllib/yt-transcribe
```

# why mlx_whisper instead of whisper.cpp?

Because it's [a lot faster on my machine](https://notes.billmill.org/link_blog/2024/08/mlx-whisper.html)

# adaptive thumbnails

By default, `-thumbs` captures a screenshot every N seconds (default 30). With `-adaptive`, the script uses FFmpeg's scene change detection to find moments where the visual content actually changes (e.g., slide transitions in a talk), and only captures thumbnails at those points.

This means:

- Slide transitions are captured precisely when they happen
- Long stretches where the speaker talks over a static slide get fewer redundant screenshots
- A fallback frame is inserted every `-maxgap` seconds (default 60) so you never go too long without a visual reference
- Duplicate detections (e.g., during animated transitions) are filtered out with `-mingap` (default 10s)

For a typical 45-minute conference talk, this produces ~40 scene-detected frames + ~35 fallback frames instead of 90 fixed-interval frames, with better alignment to actual content changes.

[Here is an example of adaptive mode](docs/klabnik-adaptive/httpswwwyoutubecomwatchvL2AOrseB2Y.html)
