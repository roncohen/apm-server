package transaction

import (
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transformations     = monitoring.NewInt(transactionMetrics, "transformations")
	transactionCounter  = monitoring.NewInt(transactionMetrics, "counter")
	spanCounter         = monitoring.NewInt(transactionMetrics, "spans")
	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
	processorSpanEntry  = common.MapStr{"name": processorName, "event": spanDocType}
)

type Payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []*Transaction
}

func DecodePayload(raw map[string]interface{}) ([]*Transaction, error) {
	if raw == nil {
		return nil, nil
	}
	// pa := &Payload{}

	var err error
	// service, err := m.DecodeService(raw["service"], err)
	// if service != nil {
	// 	pa.Service = *service
	// }
	// pa.System, err = m.DecodeSystem(raw["system"], err)
	// pa.Process, err = m.DecodeProcess(raw["process"], err)
	// pa.User, err = m.DecodeUser(raw["user"], err)
	// if err != nil {
	// 	return nil, err
	// }
	// var tx *Transaction

	decoder := utility.ManualDecoder{}
	txs := decoder.InterfaceArr(raw, "transactions")
	err = decoder.Err
	transactions := make([]*Transaction, len(txs))
	for idx, tx := range txs {
		transactions[idx], err = DecodeTransaction(tx, err)
	}
	return transactions, err
}

// func decodeSpans(input interface{}, err error) ([]m.Transformable, error) {
// 	if input == nil || err != nil {
// 		return nil, err
// 	}
// 	raw, ok := input.(map[string]interface{})
// 	if !ok {
// 		return nil, errors.New("Invalid type for transaction event")
// 	}

// 	var span *Span
// 	decoder := utility.ManualDecoder{}
// 	spans := decoder.InterfaceArr(raw, "spans")
// 	transformables := make([]m.Transformable, len(spans))
// 	for idx, sp := range tx.spans {
// 		span, err = DecodeSpan(sp, err)
// 		if err == nil {
// 			// v1 intake doesn't require a transaction ID on spans
// 			// so we set it here
// 			if span.TransactionId == nil {
// 				span.TransactionId = &e.Id
// 			}

// 			// v1 intake doesn't require a timestamp on spans
// 			// so we set it here
// 			if span.Timestamp == nil {
// 				ts := e.Timestamp.Add(time.Millisecond * time.Duration(span.Start))
// 				span.Timestamp = &ts
// 			}
// 		}

// 		transformables[idx] = span
// 	}
// 	return nil, transformables
// }

// func (pa *Payload) Transform(conf config.TransformConfig) []beat.Event {
// 	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)
// 	transactionCounter.Add(int64(len(pa.Events)))

// 	context := m.TransformContext{
// 		Service: &pa.Service,
// 		Process: pa.Process,
// 		System:  pa.System,
// 		User:    pa.User,
// 	}

// 	var events []beat.Event
// 	for idx := 0; idx < len(pa.Events); idx++ {
// 		tx := pa.Events[idx]
// 		ev := tx.Transform(conf, &context)
// 		events = append(events, ev)

// 		spanCounter.Add(int64(len(tx.Spans)))
// 		for spIdx := 0; spIdx < len(tx.Spans); spIdx++ {
// 			sp := tx.Spans[spIdx]
// 			ev = sp.Transform(conf, &context)

// 			events = append(events, ev)
// 			tx.Spans[spIdx] = nil
// 		}
// 		pa.Events[idx] = nil
// 	}

// 	return events
// }
