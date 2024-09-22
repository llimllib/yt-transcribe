package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

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
	verbose       bool
}

type Video struct {
	sanitizedURL string
	title        string
	URL          string
}

func main() {
	opts := Options{}

	flag.BoolVar(&opts.help, "help", false, "Print usage information")
	flag.StringVar(&opts.outDir, "outdir", "/tmp/yttranscribe_cache", "The directory to put the output files in")
	flag.StringVar(&opts.outFile, "outfile", "", "The name of the output HTML file")
	flag.BoolVar(&opts.thumbs, "thumbs", false, "Enable thumbnail generation")
	flag.IntVar(&opts.thumbInterval, "thumbinterval", 30, "The interval between thumbnails, in seconds")
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

	// Verify that yt-dlp is available
	if !isInstalled("yt-dlp") {
		die(red("yt-dlp is not available, please install it.") +
			"\nhttps://github.com/yt-dlp/yt-dlp?tab=readme-ov-file#installation")
	}

	if err := os.MkdirAll(opts.outDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}
	fetcher := NewFetcher(opts, video, log)

	video.title = fetcher.getVideoTitle()
	log.Debug(fmt.Sprintf("title: %s", video.title))

	audioFile := fetcher.getAudio()
	log.Debug(fmt.Sprintf("audio file: %s", audioFile))

	if opts.thumbs {
		videoFile := fetcher.getVideo()
		log.Debug(fmt.Sprintf("video file: %s", videoFile))
	}

	// XXX: get thumbs from video here

	var transcriber Transcriber

	if exists("mlx_whisper") {
		transcriber = NewMlxWhisper(opts, video, log)
	} else {
		// XXX: add option to use mlx_whisper instead of whisper.cpp
		// XXX: test perf of each
		transcriber = NewWhisper(opts, video, log)
	}
	transcriber.Transcribe(audioFile)

	// TODO: output something
	var formatter Formatter

	// XXX: add console formatter?
	formatter = NewHTMLFormatter(opts, video, transcriber, log)
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

func sanitizeURL(url string) string {
	return regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(url, "")
}

func sh(log *Log, name string, args ...string) string {
	log.Debug(append([]string{name}, args...)...)
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
	return strings.TrimSpace(string(output))
}

type Fetcher struct {
	log   *Log
	opts  Options
	video Video
}

func NewFetcher(opts Options, video Video, log *Log) *Fetcher {
	return &Fetcher{
		log:   log,
		opts:  opts,
		video: video,
	}
}

func (t Fetcher) getVideoTitle() string {
	titleFile := filepath.Join(t.opts.cacheDir, fmt.Sprintf("title_%s.txt", t.video.sanitizedURL))
	if exists(titleFile) {
		return string(must1(os.ReadFile(titleFile)))
	}

	t.log.Info("getting title")
	title := sh(t.log, "yt-dlp", "--skip-download", "--print", "title", t.video.URL)

	file := must1(os.Create(titleFile))
	defer file.Close()
	must1(file.WriteString(title))
	return title
}

// exists returns true if a file exists
func exists(f string) bool {
	_, err := os.Stat(f)
	return !os.IsNotExist(err)
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
		"--postprocessor-args", "-ar 16000 -ac 1",
		t.video.URL)

	return audioFile
}

// getVideo downloads the video's video stream with yt-dlp and returns
// the file name of the output
func (t Fetcher) getVideo() string {
	// TODO: I don't think the %(ext)s format is right here. FIXME
	videoFile := filepath.Join(t.opts.outDir, fmt.Sprintf("rawvideo_%s.%%(ext)s", t.video.sanitizedURL))
	if exists(videoFile) {
		return videoFile
	}

	t.log.Info("downloading video")
	sh(t.log, "yt-dlp", "-f", "bv", t.video.URL, "-o", videoFile)
	return videoFile
}

type Transcriber interface {
	Transcribe(audioFile string)
	GetSegments(start, end int64) []string
	GetFullText() string
}

type MlxWhisper struct {
	opts  Options
	video Video
	log   *Log
}

func NewMlxWhisper(opts Options, video Video, log *Log) *MlxWhisper {
	return &MlxWhisper{opts, video, log}
}

func (w MlxWhisper) Transcribe(audioFile string) {
	cmd := exec.Command("mlx_whisper",
		"--model", "mlx-community/distil-whisper-large-v3",
		"-f", "txt",
		"-o", filepath.Join(w.opts.cacheDir, fmt.Sprintf("audio_%s.json", w.video.sanitizedURL)),
		"--verbose", "false",
		audioFile)
	cmd.Run()
	// TODO: the transcript is now in cacheDir/audio_%s.json.txt. Save a reference to it somewhere here maybe?
	// this is where I left off
}

func (w MlxWhisper) GetSegments(start, end int64) []string {
	return []string{"TODO"}
}

func (w MlxWhisper) GetFullText() string {
	return "TODO"
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
		whisper: whisper.New(model, false),
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
func (w *Whisper) Transcribe(audioFile string) {
	// It would be cool to have progress shown by whisper, but it absolutely
	// tanks performance if you do. Citation:
	// https://github.com/ggerganov/whisper.cpp/discussions/312#discussioncomment-6318849
	w.log.Info("transcribing")

	fh := must1(os.Open(audioFile))
	samples := must1(readWav(fh))
	segments := must1(w.whisper.Transcribe(samples, runtime.NumCPU()))
	w.segments = segments

	w.log.Info("transcription complete")
}

// GetSegments returns a string representing the concatenated text of every
// segment whose start is in [start, end).
//
// start and end are given as microseconds from the start of the audio
func (w Whisper) GetSegments(start, end int64) []string {
	texts := []string{}
	for _, seg := range w.segments {
		segStart := seg.Start.Microseconds()
		if segStart >= start && segStart < end {
			texts = append(texts, seg.Text)
		}
	}
	return texts
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
  justify-content: center;
  color: #333;
  max-width: 800px;
  padding-top: 100px;
  padding-left: 20px;
  padding-right: 20px;
}
p {
  font-size: 18px;
  line-height: 30px;
  word-wrap: break-word;
  overflow-wrap: break-word;
  hyphens: auto;
}
</style>
<title>%s - transcription by yt-transcribe</title>
</head><body><p><em>transcription of <a href="%s">%s</a></em><p>
`, h.video.title, h.video.URL, h.video.title))
	// TODO: handle thumbs case
	must1(fmt.Fprint(transcriptHTML, h.transcriber.GetFullText()))
	must1(fmt.Fprintf(transcriptHTML, `<p><em><a href="https://github.com/llimllib/yt-transcribe">generated by yt-transcribe</a></em></body>`))

	return transcriptPath
}
