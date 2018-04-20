package beater

import (
	"io"
	"net/http"

	conf "github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	er "github.com/elastic/apm-server/processor/error"
	errorsSchema "github.com/elastic/apm-server/processor/error/generated-schemas"
	tr "github.com/elastic/apm-server/processor/transaction"
	transationSchema "github.com/elastic/apm-server/processor/transaction/generated-schemas"
	"github.com/pkg/errors"
)

var errorSchema = pr.CreateSchema(errorsSchema.ErrorSchema, "error")
var transactionSchema = pr.CreateSchema(transationSchema.TransactionSchema, "transaction")
var spanSchema = pr.CreateSchema(transationSchema.SpanSchema, "transaction")

// func validator()
var (
	errNoTypeProperty = errors.New("missing 'type' property")
	errUnknownType    = errors.New("unknown 'type' supplied")
)

func validate(objType string, value interface{}) error {
	switch objType {
	case "error":
		return errorSchema.ValidateInterface(value)
	case "transaction":
		return transactionSchema.ValidateInterface(value)
	case "span":
		return spanSchema.ValidateInterface(value)
	default:
		return errUnknownType
	}
}

func decode(objType string, value interface{}) (model.Transformable, error) {
	var err error
	switch objType {
	case "error":
		return er.DecodeEvent(value, err)
	case "transaction":
		return tr.DecodeTransaction(value, err)
	case "span":
		return tr.DecodeSpan(value, err)
	default:
		return nil, errUnknownType
	}
}

func batchedStreamProcessing(r *http.Request, decoder decoder.Decoder, batchSize int) ([]model.Transformable, error) {
	batch := make([]model.Transformable, 0, batchSize)
	for {
		item, err := getFromStream(r, decoder)
		if err != nil {
			return batch, err
		}

		batch = append(batch, item)

		if len(batch) >= batchSize {
			return batch, nil
		}
	}
}

func getFromStream(r *http.Request, decoder decoder.Decoder) (model.Transformable, error) {
	sr, err := decoder(r)
	if err != nil {
		return nil, err
	}

	objType, ok := sr["type"].(string)
	if !ok {
		return nil, errNoTypeProperty
	}

	err = validate(objType, sr)
	if err != nil {
		return nil, err
	}

	return decode(objType, sr)
}

func processStreamRequest(transformBatchSize int, config conf.TransformConfig, report reporter, decoder decoder.Decoder) http.Handler {
	sthandler := func(r *http.Request) serverResponse {
		rawHeader, err := decoder(r)
		if err != nil {
			return serverResponse{err, 400, nil}
		}

		htype, ok := rawHeader["type"]
		if !ok || htype != "header" {
			return cannotValidateResponse(errors.New("missing header"))
		}

		transformContext, err := model.DecodeContext(rawHeader, err)

		for {
			batch, err := batchedStreamProcessing(r, decoder, v2TransformBatchSize)

			if err == io.EOF {
				return okResponse
			}

			report(pendingReq{
				batch, config, transformContext,
			})
		}

		return okResponse
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		sendStatus(w, r, sthandler(r))
	}

	return http.HandlerFunc(handler)
}
