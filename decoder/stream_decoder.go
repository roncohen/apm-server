package decoder

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
)

type StreamDecoder func(*http.Request) (*StreamReader, error)

type StreamReader struct {
	stream *bufio.Reader
}

func (sr *StreamReader) Read() (map[string]interface{}, error) {
	buf, err := sr.stream.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	return DecodeJSONData(tmpreader)
}

func StreamDecodeLimitJSONData(maxSize int64) StreamDecoder {
	return func(req *http.Request) (*StreamReader, error) {
		reader, err := readRequestJSONData(maxSize)(req)
		if err != nil {
			return nil, err
		}
		sr := &StreamReader{bufio.NewReader(reader)}

		return sr, nil
	}
}

func DecoderStreamAdapter(sd StreamDecoder) Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		var sr *StreamReader
		var err error

		if sr == nil {
			sr, err = sd(req)
			if err != nil {
				return nil, err
			}
		}
		return sr.Read()
	}
}
