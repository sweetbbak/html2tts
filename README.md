## Grab text from any URL and play it with TTS

```sh
  ./html2tts --url "https://www.novel.com/novel"
```

html2tts has a built in audio player however at the current moment,
piper-tts and a voice model is needed. I'd like to add more options. It
also requires Curl because bypassing cloudflare using Go is a giant feat.
