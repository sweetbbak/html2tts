package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/ebitengine/oto/v3"
	"golang.org/x/net/html"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// ----------- HTML  ------------------------------------------
func request(url string) string {
	cc := "bash"
	dash_c := "-c"
	args := "curl -fsSL --tls13-ciphers TLS_AES_128_GCM_SHA256 --tlsv1.3 -A"
	userAgent := "'Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/118.0'"
	kit := strings.Join([]string{args, userAgent, url}, " ")
	im := exec.Command(cc, dash_c, kit)
	stdout, err := im.Output()
	if err != nil {
		fmt.Println("err: ", err)
	}
	// fmt.Println(string(stdout))
	return string(stdout)
}

func Body(doc *html.Node) (*html.Node, error) {
	var body *html.Node
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "p" {
			body = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(doc)
	if body != nil {
		return body, nil
	}
	return nil, errors.New("Missing <body> in the node tree")
}

func renderNode(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}

func get_printable(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, text)
	return text
}

func rm_symbols(s string) string {
	// re := regexp.MustCompile("[[:^ascii:]]")
	// re := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	re := regexp.MustCompile(`[^a-zA-Z0-9 \!\?\,\.\(\)]+`)
	t := re.ReplaceAllLiteralString(s, "")
	return t
}

func escape_string(input string) string {
	input = strings.Replace(input, "'", "", -1)
	input = strings.Replace(input, "|", "", -1)
	input = strings.Replace(input, "<", "", -1)
	input = strings.Replace(input, ">", "", -1)
	input = strings.Replace(input, "*", "", -1)
	input = strings.Replace(input, "--", "", -1)
	input = strings.Replace(input, "\\", "", -1)
	input = strings.Replace(input, "\"", "", -1)
	input = strings.Replace(input, "\n\n", "\n", -1)
	input = strings.Replace(input, "  ", " ", -1)
	// input = strings.Replace(input, "\n", " ", -1)
	// input = strings.TrimSpace(input)

	return input
}

// ----------- AUDIO ------------------------------------------
func init_oto() *oto.Context {

	op := &oto.NewContextOptions{}
	// Usually 44100 or 48000. Other values might cause distortions in Oto
	op.SampleRate = 22050
	// Number of channels (aka locations) to play sounds from. Either 1 or 2.
	// 1 is mono sound, and 2 is stereo (most speakers are stereo).
	op.ChannelCount = 1
	// Format of the source. go-mp3's format is signed 16bit integers.
	op.Format = oto.FormatSignedInt16LE
	// Remember that you should **not** create more than one context
	otoCtx, readyChan, err := oto.NewContext(op)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	// It might take a bit for the hardware audio devices to be ready, so we wait on the channel.
	<-readyChan

	return otoCtx
}

func play(otoCtx *oto.Context, out io.Reader, done chan bool) {
	player := otoCtx.NewPlayer(out)
	// Play starts playing the sound and returns without waiting for it (Play() is async).
	player.Play()
	// We can wait for the sound to finish playing using something like this
	res := <-done
	for player.IsPlaying() && res != true {
		time.Sleep(time.Millisecond)
		res = <-done
	}

	done <- true
	err := player.Close()
	if err != nil {
		panic("player.Close failed: " + err.Error())
	}
}
func piper_tts(text string) (io.Reader, error) {
	// text = escape_string(text)
	// fileName := hashString(input) + ".wav"
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
	}

	voice := cwd + "/models/amy.onnx"
	// cmd := exec.Command(piper_bin, "--model", voice, "--output_file", "-")
	cmd := exec.Command(piper_bin, "--model", voice, "--output_raw")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	_, err = io.WriteString(stdin, text)
	if err != nil {
		return nil, err
	}

	stdin.Close()
	return stdoutPipe, nil

}

func find_piper() string {
	var piper_bin string
	piper_bin, err := exec.LookPath("piper-tts")
	if err != nil {
		fmt.Println(err)
	}

	if piper_bin == "" {
		piper_bin, err = exec.LookPath("piper")
		if err != nil {
			fmt.Println(err)

			if piper_bin == "" {
				fmt.Println("Couldnt find piper binary...")
				os.Exit(0)
			}
		}
	}
	return piper_bin
}

var piper_bin = find_piper()

// ----------- FLAGS ------------------------------------------

// ----------- MAIN  ------------------------------------------
func main() {
	var url string
	flag.StringVar(&url, "url", "", "url to parse")
	flag.StringVar(&url, "u", "", "url to parse")

	flag.Parse()

	// out := request("https://tagnovel.com/revenge-of-the-iron-blooded-sword-hound-chapter-89/")
	// out := request("https://www.scribblehub.com/read/861263-erorealm--my-new-life-as-a-lewd-succibus-queen-/chapter/863867/")
	if url == "" {
		fmt.Println("No URL provided")
		os.Exit(1)
	}

	out := request(url)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(out))
	if err != nil {
		fmt.Println(err)
	}

	var text []string
	// find all "p" tags and iterate + print their text
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		t := s.Text()
		text = append(text, t)
	})
	// fmt.Println(text)

	for x := range text {
		text[x] = get_printable(text[x])
		text[x] = rm_symbols(text[x])
	}

	var str strings.Builder
	for x := range text {
		// fmt.Println(text[x])
		str.WriteString(text[x])
		str.WriteString(" ")
	}

	// fmt.Println(escape_string(str.String()))
	ttstext := escape_string(str.String())

	outfile, err := os.CreateTemp("/tmp", "tts.txt")
	if err != nil {
		fmt.Println(err)
	}

	defer outfile.Close()
	outfile.Write([]byte(ttstext))

	output, err := piper_tts(ttstext)
	if err != nil {
		fmt.Println(err)
	}

	otoCtx := init_oto()
	// play(otoCtx, output)
	done := make(chan bool)

	go func() {
		play(otoCtx, output, done)
	}()

	// stop playback
	// go func() {
	// 	time.Sleep(time.Second * 7)
	// 	done <- true
	// 	otoCtx.Suspend()
	// }()

	// wait til done
	<-done
}
