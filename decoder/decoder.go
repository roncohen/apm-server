package decoder

import (
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/elastic/beats/libbeat/common"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/monitoring"
)

type Reader func(req *http.Request) (io.ReadCloser, error)

// type Decoder func(req *http.Request) (map[string]interface{}, error)

var (
	decoderMetrics                = monitoring.Default.NewRegistry("apm-server.decoder", monitoring.PublishExpvar)
	missingContentLengthCounter   = monitoring.NewInt(decoderMetrics, "missing-content-length.count")
	deflateLengthAccumulator      = monitoring.NewInt(decoderMetrics, "deflate.content-length")
	deflateCounter                = monitoring.NewInt(decoderMetrics, "deflate.count")
	gzipLengthAccumulator         = monitoring.NewInt(decoderMetrics, "gzip.content-length")
	gzipCounter                   = monitoring.NewInt(decoderMetrics, "gzip.count")
	uncompressedLengthAccumulator = monitoring.NewInt(decoderMetrics, "uncompressed.content-length")
	uncompressedCounter           = monitoring.NewInt(decoderMetrics, "uncompressed.count")
	readerAccumulator             = monitoring.NewInt(decoderMetrics, "reader.size")
	readerCounter                 = monitoring.NewInt(decoderMetrics, "reader.count")
)

type monitoringReader struct {
	r io.ReadCloser
}

func (mr monitoringReader) Read(p []byte) (int, error) {
	n, err := mr.r.Read(p)
	readerAccumulator.Add(int64(n))
	return n, err
}

func (mr monitoringReader) Close() error {
	return mr.r.Close()
}

func DecodeLimitJSONData(maxSize int64) V1Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		reader, err := readRequestJSONData(maxSize)(req)
		if err != nil {
			return nil, err
		}

		return DecodeJSONData(monitoringReader{reader})
	}
}

func getDecompressionReader(req *http.Request) (io.ReadCloser, error) {
	reader := req.Body
	if reader == nil {
		return nil, errors.New("no content")
	}

	cLen := req.ContentLength
	knownCLen := cLen > -1
	if !knownCLen {
		missingContentLengthCounter.Inc()
	}
	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		if knownCLen {
			deflateLengthAccumulator.Add(cLen)
			deflateCounter.Inc()
		}
		var err error
		reader, err = zlib.NewReader(reader)
		if err != nil {
			return nil, err
		}

	case "gzip":
		if knownCLen {
			gzipLengthAccumulator.Add(cLen)
			gzipCounter.Inc()
		}
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
	default:
		if knownCLen {
			uncompressedLengthAccumulator.Add(cLen)
			uncompressedCounter.Inc()
		}
	}
	return reader, nil
}

// readRequestJSONData makes a function that uses information from an http request to construct a Limited ReadCloser
// of json data from the body of the request
func readRequestJSONData(maxSize int64) Reader {
	return func(req *http.Request) (io.ReadCloser, error) {
		contentType := req.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

		reader, err := getDecompressionReader(req)
		if err != nil {
			return nil, err
		}

		readerCounter.Inc()
		return http.MaxBytesReader(nil, reader, maxSize), nil
	}
}

// readRequestNDJSONData makes a function that uses information from an http request to construct a Limited ReadCloser
// of json data from the body of the request
func readRequestNDJSONData(maxSize int64) Reader {
	return func(req *http.Request) (io.ReadCloser, error) {
		contentType := req.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/ndjson") {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

		reader, err := getDecompressionReader(req)
		if err != nil {
			return nil, errors.Wrap(err, "getting decompression reader")
		}

		readerCounter.Inc()
		return http.MaxBytesReader(nil, reader, maxSize), nil
	}
}

func DecodeJSONData(reader io.ReadCloser) (map[string]interface{}, error) {
	v := make(map[string]interface{})
	d := json.NewDecoder(reader)
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		// If we run out of memory, for example
		return nil, errors.Wrap(err, "data read error")
	}
	return v, nil
}

func DecodeSourcemapFormData(req *http.Request) (map[string]interface{}, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	file, _, err := req.FormFile("sourcemap")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sourcemapBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"sourcemap":       string(sourcemapBytes),
		"service_name":    req.FormValue("service_name"),
		"service_version": req.FormValue("service_version"),
		"bundle_filepath": utility.CleanUrlPath(req.FormValue("bundle_filepath")),
	}

	return payload, nil
}

type Extractor func(req *http.Request) map[string]interface{}

func UserExtractor(req *http.Request) map[string]interface{} {
	m := map[string]interface{}{
		"user-agent": req.Header.Get("User-Agent"),
	}
	if ip := utility.ExtractIP(req); net.ParseIP(ip) != nil {
		m["ip"] = ip
	}

	return map[string]interface{}{
		"user": m,
	}
}

func SystemExtractor(req *http.Request) map[string]interface{} {
	if ip := utility.ExtractIP(req); net.ParseIP(ip) != nil {
		return map[string]interface{}{
			"system": map[string]interface{}{"ip": ip},
		}
	}

	return map[string]interface{}{}
}

func GetAugmenter(req *http.Request, extractors []Extractor) Augmenter {
	extra := make([]map[string]interface{}, len(extractors))
	for idx, extractor := range extractors {
		extra[idx] = extractor(req)
	}

	return Augmenter{extra}
}

type Augmenter struct {
	extra []map[string]interface{}
}

func (a *Augmenter) Augment(input common.MapStr) {
	for _, e := range a.extra {
		for key, v := range e {
			val := v.(map[string]interface{})
			utility.MergeAdd(input, key, val)
		}
	}
}

// func DecodeUserData(decoder StreamDecoder, enabled bool) StreamDecoder {
// 	if !enabled {
// 		return decoder
// 	}

// 	augment := func(req *http.Request) map[string]interface{} {
// 		return map[string]interface{}{
// 			"ip":         utility.ExtractIP(req),
// 			"user-agent": req.Header.Get("User-Agent"),
// 		}
// 	}
// 	return augmentData(decoder, "user", augment)
// }

// func DecodeSystemData(decoder StreamDecoder, enabled bool) StreamDecoder {
// 	if !enabled {
// 		return decoder
// 	}

// 	augment := func(req *http.Request) map[string]interface{} {
// 		return map[string]interface{}{"ip": utility.ExtractIP(req)}
// 	}
// 	return augmentData(decoder, "system", augment)
// }

// func AugmenterHandler(decoder StreamDecoder, key string, extract func(req *http.Request) map[string]interface{}) StreamDecoder {
// 	return func(req *http.Request) (EntityStreamReader, error) {
// 		decoder, err := decoder(req)
// 		if err != nil {
// 			return v, err
// 		}

// 		val := extract(req)

// 		return func() (map[string]interface{}, error) {
// 			utility.InsertInMap(v, key)
// 		}

// 		return v, nil
// 	}
// }

// func Augmenter() {
// 	return func(req *http.Request) {

// 	}
// }
