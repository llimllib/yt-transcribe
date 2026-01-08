package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-audio/wav"
	"github.com/llimllib/yt-transcribe/whisper"
)

func usage() {
	fmt.Println(`Usage: yt-transcribe [options] <youtube-url>
Transcribe a youtube video into an easily readable HTML file

OPTIONS
  -help:          print this message
  -outdir:        the directory to put the output files in. [default /tmp/yttranscribe_cache]
  -outfile:       the name of the output HTML file
  -thumbs:        enable thumbnail generation
  -thumbinterval: the interval between thumbnails, in seconds [default 30]
  -v:             print more verbose output

EXAMPLES
Transcribe a youtube video:
    yt-transcribe 'https://www.youtube.com/watch?v=vP4iY1TtS3s'

Transcribe a video and insert thumbnails every 30 seconds (the default):
    yt-transcribe -thumbs 'https://www.youtube.com/watch?v=Ac7G7xOG2Ag'

Transcribe a video and insert thumbnails every 10 seconds:
    yt-transcribe -thumbs -thumbinterval 10 'https://www.youtube.com/watch?v=X48G7Y0VWW4'

Transcribe a video to the 'look-around-you' directory, with a filename 'water.html':
    yt-transcribe -thumbs -outdir ./look-around-you -outfile water.html 'https://www.youtube.com/watch?v=gaI6kBVyu00'

source: https://github.com/llimllib/yt-transcribe`)
	os.Exit(0)
}

type Options struct {
	cacheDir      string
	help          bool
	outDir        string
	outFile       string
	thumbs        bool
	thumbInterval int
	thumbWidth    int
	verbose       bool
}

type Video struct {
	// duration is the video duration in *microseconds*
	durationMicroseconds int64
	sanitizedURL         string
	title                string
	description          string
	localFilename        string
	// thumbnails is a list of filenames for thumbnail images
	thumbnails []string
	URL        string
}

func main() {
	opts := Options{}

	flag.BoolVar(&opts.help, "help", false, "Print usage information")
	flag.StringVar(&opts.outDir, "outdir", "/tmp/yttranscribe_cache", "The directory to put the output files in")
	flag.StringVar(&opts.outFile, "outfile", "", "The name of the output HTML file")
	flag.BoolVar(&opts.thumbs, "thumbs", false, "Enable thumbnail generation")
	flag.IntVar(&opts.thumbInterval, "thumbinterval", 30, "The interval between thumbnails, in seconds")
	flag.IntVar(&opts.thumbWidth, "thumbwidth", 640, "The width of the extracted thumbnail images")
	flag.BoolVar(&opts.verbose, "v", false, "Print more verbose output")
	flag.Parse()

	if opts.help || len(flag.Args()) != 1 {
		usage()
	}

	video := Video{
		URL:          flag.Args()[0],
		sanitizedURL: sanitizeURL(flag.Args()[0]),
	}

	var log *Log
	if opts.verbose {
		log = initLog("DEBUG")
	} else {
		log = initLog("INFO")
	}

	if opts.outFile == "" {
		opts.outFile = fmt.Sprintf("%s.html", sanitizeURL(video.URL))
	}

	opts.cacheDir = "/tmp/yttranscribe_cache"

	log.Debug("flags", fmt.Sprintf("%#v", opts))

	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}
	fetcher := NewFetcher(opts, &video, log)

	fetcher.getVideoMetadata()
	log.Debug(fmt.Sprintf("title: %s", video.title))
	log.Debug(fmt.Sprintf("description: %s", video.description))

	audioFile := fetcher.getAudio()
	log.Debug(fmt.Sprintf("audio file: %s", audioFile))

	if opts.thumbs {
		video.localFilename = fetcher.getVideo()
		log.Debug(fmt.Sprintf("video file: %#v", video))
		thumbnailer := NewFFmpegThumbnailer(opts, video, log)
		video.thumbnails = thumbnailer.GetThumbnails()
	}

	var transcriber Transcriber

	if commandExists("mlx_whisper") {
		log.Debug("transcriber: mlx_whisper")
		transcriber = NewMlxWhisper(opts, video, log)
	} else {
		log.Debug("transcriber: built-in")
		transcriber = NewWhisper(opts, video, log)
	}
	transcriber.Transcribe(audioFile)

	// XXX: Should there be a "transcribe neaten" step, where we optionally use
	// an LLM to group the sentences into paragraphs so it reads better, or
	// something like that?

	// XXX: add console formatter?
	formatter := NewHTMLFormatter(opts, video, transcriber, log)
	open(formatter.Format())
}

const (
	RESET = "\x1b[0m"
	RED   = "\x1b[31m"
	GREEN = "\x1b[32m"
)

func red(msg string) string {
	return fmt.Sprintf("%s%s%s", RED, msg, RESET)
}

func green(msg string) string {
	return fmt.Sprintf("%s%s%s", GREEN, msg, RESET)
}

func die(msg string) {
	fmt.Printf("%s\n", msg)
	os.Exit(1)
}

func isInstalled(program string) bool {
	_, err := exec.LookPath(program)
	return err == nil
}

func open(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		runDll32 := filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "rundll32.exe")
		cmd = exec.Command(runDll32, "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	must(cmd.Run())
}

type Log struct {
	level int
}

// create a logger with its log level based on the LOG_LEVEL environment var,
// defaulting to INFO
func initLog(level string) *Log {
	var levelI int
	if strings.ToLower(level) == "debug" {
		levelI = 0
	} else {
		levelI = 1
	}
	return &Log{levelI}
}

func (l Log) Debug(msg ...string) {
	if l.level == 0 {
		fmt.Printf("%s\n", strings.Join(msg, " "))
	}
}

func (l Log) Info(msg ...string) {
	fmt.Printf("%s%s%s\n", GREEN, strings.Join(msg, " "), RESET)
}

func (l Log) Error(err error, context string) {
	fmt.Printf("%s%s%s\n%s\n", RED, err, RESET, context)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// must1 panics if err is non-nil and returns value otherwise
func must1[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func must2[T any, U any](val1 T, val2 U, err error) (T, U) {
	if err != nil {
		panic(err)
	}
	return val1, val2
}

func sanitizeURL(url string) string {
	return regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(url, "")
}

func sh(log *Log, name string, args ...string) string {
	log.Debug(append([]string{name}, args...)...)
	t1 := time.Now()
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Printf("%s: %s %s\n%s\n%s",
				red("failed running command"),
				name,
				strings.Join(args, " "),
				output,
				exitErr.Stderr)
		} else {
			fmt.Printf("%s: %s %s\n%s",
				red("failed running command"),
				name,
				strings.Join(args, " "),
				output)
		}
		os.Exit(1)
	}
	t2 := time.Now()
	log.Debug(fmt.Sprintf("command took: %s", t2.Sub(t1)))
	return strings.TrimSpace(string(output))
}

type Fetcher struct {
	log   *Log
	opts  Options
	video *Video
}

func NewFetcher(opts Options, video *Video, log *Log) *Fetcher {
	// Verify that yt-dlp is available
	if !isInstalled("yt-dlp") {
		die(red("yt-dlp is not available, please install it.") +
			"\nhttps://github.com/yt-dlp/yt-dlp?tab=readme-ov-file#installation")
	}

	return &Fetcher{
		log:   log,
		opts:  opts,
		video: video,
	}
}

type VideoMetadata struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Duration    float64 `json:"duration"` // duration in seconds
	UploadDate  string  `json:"upload_date"`
	Uploader    string  `json:"uploader"`
	Channel     string  `json:"channel"`
}

func (t *Fetcher) getVideoMetadata() {
	metadataFile := filepath.Join(t.opts.cacheDir, fmt.Sprintf("metadata_%s.json", t.video.sanitizedURL))

	// Check if we have cached metadata
	if exists(metadataFile) {
		var metadata VideoMetadata
		must(json.Unmarshal(must1(os.ReadFile(metadataFile)), &metadata))
		t.video.title = metadata.Title
		t.video.description = metadata.Description
		t.video.durationMicroseconds = int64(metadata.Duration * 1_000_000)
		return
	}

	t.log.Info("getting video metadata")
	// Get all metadata in a single yt-dlp call, outputting as JSON
	output := sh(t.log, "yt-dlp", "--skip-download", "--print",
		"{\"title\":%(title)j,\"description\":%(description)j,\"duration\":%(duration)j,\"upload_date\":%(upload_date)j,\"uploader\":%(uploader)j,\"channel\":%(channel)j}",
		t.video.URL)

	var metadata VideoMetadata
	must(json.Unmarshal([]byte(output), &metadata))

	t.video.title = metadata.Title
	t.video.description = metadata.Description
	t.video.durationMicroseconds = int64(metadata.Duration * 1_000_000)

	// Cache the metadata as JSON
	metadataJSON := must1(json.MarshalIndent(metadata, "", "  "))
	must(os.WriteFile(metadataFile, metadataJSON, 0o644))
}

// exists returns true if a file exists
func exists(f string) bool {
	_, err := os.Stat(f)
	return !os.IsNotExist(err)
}

// commandExists returns true if an executable named 'file' exists on PATH.
// Disregard any errors.
func commandExists(file string) bool {
	path, err := exec.LookPath(file)
	return err == nil && path != ""
}

// getAudio downloads the video's audio stream with yt-dlp and returns
// the file name of the output
func (t Fetcher) getAudio() string {
	audioFile := filepath.Join(t.opts.cacheDir, fmt.Sprintf("rawaudio_%s.wav", t.video.sanitizedURL))
	if exists(audioFile) {
		return audioFile
	}

	t.log.Info("downloading audio")
	sh(t.log, "yt-dlp",
		"--extract-audio",
		"--audio-format", "wav",
		"-o", audioFile,
		// tell yt-dlp to use ffmpeg to set the sample rate to 16khz and
		// channels to 1, the format required by whisper.cpp
		// XXX: does this introduce a dependency on ffmpeg, does yt-dlp link
		// it, or does yt-dlp already depend on it? Unclear to me
		"--postprocessor-args", "-ar 16000 -ac 1",
		t.video.URL)

	return audioFile
}

// getVideo downloads the video's video stream with yt-dlp and returns
// the file name of the output
func (t *Fetcher) getVideo() string {
	// Check if we already have a downloaded video file
	videoPattern := filepath.Join(t.opts.outDir, fmt.Sprintf("rawvideo_%s.*", t.video.sanitizedURL))
	files := must1(filepath.Glob(videoPattern))

	// If we don't already have the raw video, download it
	if len(files) == 0 {
		t.log.Info("downloading video")
		videoTemplate := filepath.Join(t.opts.outDir, fmt.Sprintf("rawvideo_%s.%%(ext)s", t.video.sanitizedURL))
		sh(t.log, "yt-dlp", "-f", "bv", t.video.URL, "-o", videoTemplate)
	}

	// Get the filename of the downloaded file - yt-dlp has replaced %(ext)s
	// with the video's extension
	files = must1(filepath.Glob(videoPattern))
	if len(files) == 0 {
		die(red(fmt.Sprintf("failed to find downloaded video file matching %s", videoPattern)))
	}

	return files[0]
}

type Thumbnailer interface {
	// GetThumbnails takes an interval on which to pull thumbnails, and returns
	// a list of filenames for the extracted thumbnails
	GetThumbnails() []string
}

type FFmpegThumbnailer struct {
	opts  Options
	video Video
	log   *Log
}

func NewFFmpegThumbnailer(opts Options, video Video, log *Log) *FFmpegThumbnailer {
	return &FFmpegThumbnailer{
		opts:  opts,
		video: video,
		log:   log,
	}
}

func (f *FFmpegThumbnailer) GetThumbnails() []string {
	f.log.Info("extracting thumbnails")

	// Create thumbnails directory
	thumbDir := filepath.Join(f.opts.cacheDir, fmt.Sprintf("thumbs_%s", f.video.sanitizedURL))
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		f.log.Error(err, "failed to create thumbnail directory")
		return []string{}
	}

	thumbnails := []string{}

	// convert duration from microseconds to seconds
	duration := float64(f.video.durationMicroseconds) / 1_000_000
	f.log.Info("duration", fmt.Sprintf("%f", duration), fmt.Sprintf("%d", f.video.durationMicroseconds))

	// Extract thumbnails at each interval
	for i := 0.0; i < duration; i += float64(f.opts.thumbInterval) {
		thumbFile := filepath.Join(thumbDir, fmt.Sprintf("thumb_%06d.jpg", int(i)))

		// Skip if thumbnail already exists
		if exists(thumbFile) {
			thumbnails = append(thumbnails, thumbFile)
			continue
		}

		// Extract thumbnail at this timestamp
		// Use i+1 to skip past potential black frames at segment boundaries
		timestamp := i + 1.0
		if timestamp >= duration {
			timestamp = duration - 1.0
		}

		// Use video filter to:
		// - Select better frames (I-frames or frames with scene changes)
		// - Scale to desired width while maintaining aspect ratio
		// - Apply unsharp mask for better quality
		vfilter := fmt.Sprintf("select='eq(n,0)+eq(pict_type,PICT_TYPE_I)+gt(scene,0.3)',scale=%d:-1,unsharp", f.opts.thumbWidth)

		sh(f.log, "ffmpeg",
			"-ss", fmt.Sprintf("%.2f", timestamp),
			"-i", f.video.localFilename,
			"-vf", vfilter,
			"-vframes", "1",
			"-q:v", "2",
			"-y",
			thumbFile)

		thumbnails = append(thumbnails, thumbFile)
	}

	f.log.Info(fmt.Sprintf("extracted %d thumbnails", len(thumbnails)))
	return thumbnails
}

type Segment struct {
	Start float64
	End   float64
	Text  string
}

type Transcriber interface {
	Transcribe(audioFile string)
	GetSegments(start, end int64) []Segment
	GetFullText() string
}

type MlxSegment struct {
	ID    float64 `json:"id"`
	Seek  float64 `json:"seek"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type MlxJSON struct {
	Text     string       `json:"text"`
	Language string       `json:"language"`
	Segments []MlxSegment `json:"segments"`
}

type MlxWhisper struct {
	opts           Options
	video          Video
	log            *Log
	transcriptFile string
}

func NewMlxWhisper(opts Options, video Video, log *Log) *MlxWhisper {
	return &MlxWhisper{opts: opts, video: video, log: log}
}

// getWavDuration returns the duration of wavFile in seconds
func getWavDuration(wavFile string) int {
	file := must1(os.Open(wavFile))
	defer file.Close()

	return int(math.Ceil(must1(wav.NewDecoder(file).Duration()).Seconds()))
}

func (w *MlxWhisper) Transcribe(audioFile string) {
	// mlx_whisper doesn't let you control the exact output filename; instead
	// it outputs to the specified output directory with a modified version of
	// the file name - replacing the extension with .json
	outfile := filepath.Join(
		filepath.Dir(audioFile),
		filepath.Base(audioFile[:len(audioFile)-len(filepath.Ext(audioFile))])+".json")
	w.log.Debug("output file:", outfile)
	w.transcriptFile = outfile

	if w.opts.thumbs {
		intervals := []string{"0", strconv.Itoa(w.opts.thumbInterval)}
		i := w.opts.thumbInterval
		for i < getWavDuration(audioFile) {
			intervals = append(intervals, strconv.Itoa(i), strconv.Itoa(i+w.opts.thumbInterval))
			i += w.opts.thumbInterval
		}
		intervalStr := strings.Join(intervals, ",")

		sh(w.log, "mlx_whisper",
			"--model", "mlx-community/distil-whisper-large-v3",
			"-f", "json",
			"-o", w.opts.cacheDir,
			"--verbose", "False",
			"--clip-timestamps", intervalStr,
			audioFile)
	} else {
		sh(w.log, "mlx_whisper",
			"--model", "mlx-community/distil-whisper-large-v3",
			"-f", "json",
			"-o", w.opts.cacheDir,
			"--verbose", "False",
			audioFile)
	}
}

// GetSegments returns a string representing the concatenated text of every
// segment whose start is in [start, end).
//
// start and end are given as microseconds from the start of the audio
// TODO: add the ability to cut video length
func (w MlxWhisper) GetSegments(start, end int64) []Segment {
	var whisperData MlxJSON
	w.log.Debug("attempting to open", w.transcriptFile)
	must(json.Unmarshal(must1(os.ReadFile(w.transcriptFile)), &whisperData))
	segments := []Segment{}
	for _, segment := range whisperData.Segments {
		segments = append(segments, Segment{
			Start: segment.Start,
			End:   segment.End,
			Text:  segment.Text,
		})
	}
	return segments
}

func (w MlxWhisper) GetFullText() string {
	var whisperData MlxJSON
	must(json.Unmarshal(must1(os.ReadFile(w.transcriptFile)), &whisperData))
	return whisperData.Text
}

type Whisper struct {
	log      *Log
	whisper  *whisper.Whisper
	segments []whisper.Segment
}

func NewWhisper(opts Options, video Video, log *Log) *Whisper {
	// TODO: get this from... where? I think maybe I have file downloading
	// logic in blisper I could copy?
	model := filepath.Join(must1(user.Current()).HomeDir, ".local/share/blisper/ggml-large.bin")
	return &Whisper{
		log:     log,
		whisper: whisper.New(model, !opts.verbose),
	}
}

// readWav reads a wav file and returns its decoded data or an error
func readWav(fh *os.File) ([]float32, error) {
	dec := wav.NewDecoder(fh)
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, err
	} else if dec.SampleRate != whisper.SAMPLE_RATE {
		return nil, fmt.Errorf("unsupported sample rate: %d", dec.SampleRate)
	} else if dec.NumChans != 1 {
		return nil, fmt.Errorf("unsupported number of channels: %d", dec.NumChans)
	}
	return buf.AsFloat32Buffer().Data, nil
}

// XXX: idea, can we use purego here to avoid cgo and be able to cross-compile
// to windows? https://github.com/ebitengine/purego

// Transcribe transcribes an audio file into w.segments
func (w *Whisper) Transcribe(audioFile string) {
	// It would be cool to have progress shown by whisper, but it absolutely
	// tanks performance if you do. Citation:
	// https://github.com/ggerganov/whisper.cpp/discussions/312#discussioncomment-6318849
	w.log.Info("transcribing")
	t1 := time.Now()

	fh := must1(os.Open(audioFile))
	samples := must1(readWav(fh))
	segments := must1(w.whisper.Transcribe(samples, runtime.NumCPU()))
	w.segments = segments

	t2 := time.Now()
	w.log.Info("transcription complete:", t2.Sub(t1).String())
}

// GetSegments returns a string representing the concatenated text of every
// segment whose start is in [start, end).
//
// start and end are given as microseconds from the start of the audio
func (w Whisper) GetSegments(start, end int64) []Segment {
	segments := []Segment{}
	for _, seg := range w.segments {
		segments = append(segments, Segment{
			Start: float64(seg.Start.Microseconds()) / 1000000.0,
			End:   float64(seg.End.Microseconds()) / 1000000.0,
			Text:  seg.Text,
		})
	}
	return segments
}

func (w Whisper) GetFullText() string {
	texts := []string{}
	for _, seg := range w.segments {
		texts = append(texts, seg.Text)
	}
	return strings.Join(texts, "<p>\n")
}

type Formatter interface {
	// Format runs the formatter and returns a string... that might be a file
	// name or a string to display, depending on the formatter
	Format() string
}

// linkifyURLs finds URLs in text and converts them to HTML links
func linkifyURLs(text string) string {
	// Match URLs starting with http://, https://, or www.
	urlRegex := regexp.MustCompile(`(https?://[^\s<]+|www\.[^\s<]+)`)
	return urlRegex.ReplaceAllStringFunc(text, func(url string) string {
		href := url
		// Add https:// prefix for www. URLs
		if strings.HasPrefix(url, "www.") {
			href = "https://" + url
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, href, url)
	})
}

type HTMLFormatter struct {
	log         *Log
	opts        Options
	transcriber Transcriber
	video       Video
}

func NewHTMLFormatter(opts Options, video Video, transcriber Transcriber, log *Log) *HTMLFormatter {
	return &HTMLFormatter{
		opts:        opts,
		video:       video,
		transcriber: transcriber,
		log:         log,
	}
}

func (h HTMLFormatter) Format() string {
	transcriptPath := path.Join(h.opts.outDir, h.opts.outFile)
	transcriptHTML := must1(os.Create(transcriptPath))
	defer transcriptHTML.Close()

	h.log.Info(fmt.Sprintf("outputting %s", transcriptPath))

	must1(fmt.Fprintf(transcriptHTML, `<html><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
html {
    /* fixes font size on iOS */
    text-size-adjust: none;
    -webkit-text-size-adjust: none;
}
body {
  font-family: Georgia, "Book Antiqua", serif;
  margin: auto;
  color: #333;
  max-width: 1000px;
  padding-top: 100px;
  padding-left: 20px;
  padding-right: 20px;
  display: flex;
  flex-direction: column;
}
.content {
  display: grid;
  grid-template-columns: 80px 1fr;
  gap: 20px;
  max-width: 900px;
  margin: 0 auto;
}
.timestamp {
  text-align: right;
  padding-top: 5px;
}
.timestamp a {
  color: #999;
  text-decoration: none;
  font-size: 14px;
  font-family: monospace;
}
.timestamp a:hover {
  color: #333;
  text-decoration: underline;
}
.text {
  min-width: 0;
}
p {
  font-size: 18px;
  line-height: 30px;
  word-wrap: break-word;
  overflow-wrap: break-word;
  hyphens: auto;
  margin: 0 0 20px 0;
}
details {
  margin-top: 10px;
  margin-bottom: 20px;
  background: #ffddee;
  padding: 15px;
}
summary {
  cursor: pointer;
  font-style: italic;
}
details p {
  margin-top: 10px;
  white-space: pre-wrap;
}
</style>
<title>%s - transcription by yt-transcribe</title>
</head><body><p><em>transcription of <a href="%s">%s</a></em></p>
<details>
<summary>Video Description</summary>
<p>%s</p>
</details>
`, h.video.title, h.video.URL, h.video.title, linkifyURLs(h.video.description)))

	must1(fmt.Fprintf(transcriptHTML, "<div class=\"content\">\n"))

	// Get segments with timestamps to properly match thumbnails
	segments := h.transcriber.GetSegments(int64(0), h.video.durationMicroseconds)

	// Track which thumbnail index we're on
	thumbIndex := 0
	nextThumbTime := 0.0

	// Track the current minute marker, start at -1 so we handle minute 0
	currentMinute := -1

	for _, segment := range segments {
		segmentMinute := int(segment.Start / 60)

		// Check if we've crossed into a new minute
		if segmentMinute > currentMinute {
			// Close previous text div if this isn't the first segment
			if currentMinute >= 0 {
				must1(fmt.Fprintf(transcriptHTML, "</div>\n"))
			}

			currentMinute = segmentMinute

			// Add timestamp link
			minutes := currentMinute
			hours := minutes / 60
			mins := minutes % 60
			var timeStr string
			if hours > 0 {
				timeStr = fmt.Sprintf("%d:%02d:00", hours, mins)
			} else {
				timeStr = fmt.Sprintf("%d:00", mins)
			}
			timestamp := currentMinute * 60
			videoLink := fmt.Sprintf("%s&t=%ds", h.video.URL, timestamp)
			must1(fmt.Fprintf(transcriptHTML, "<div class=\"timestamp\"><a href=\"%s\">%s</a></div><div class=\"text\">\n", videoLink, timeStr))
		}

		// Check if we should insert a thumbnail before this segment
		// Thumbnails are at intervals of thumbInterval seconds
		if h.video.thumbnails != nil && thumbIndex < len(h.video.thumbnails) {
			// If this segment starts at or after the next thumbnail time, insert the thumbnail
			if segment.Start >= nextThumbTime {
				// Create a link to the YouTube video at this timestamp
				// YouTube accepts timestamps in the format: &t=XXs (seconds)
				timestamp := int(nextThumbTime)
				videoLink := fmt.Sprintf("%s&t=%ds", h.video.URL, timestamp)
				must1(fmt.Fprintf(transcriptHTML, "<a href=\"%s\"><img src=\"%s\"></a>", videoLink, h.video.thumbnails[thumbIndex]))
				thumbIndex++
				nextThumbTime = float64(thumbIndex * h.opts.thumbInterval)
			}
		}
		must1(fmt.Fprintf(transcriptHTML, "<p>%s</p>\n", segment.Text))
	}

	// Close the last text div and content div
	must1(fmt.Fprintf(transcriptHTML, "</div></div>\n"))
	must1(fmt.Fprintf(transcriptHTML, `<p><em><a href="https://github.com/llimllib/yt-transcribe">generated by yt-transcribe</a></em></body>`))

	return transcriptPath
}
