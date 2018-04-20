package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/tests/loader"
)

func main() {

	err := generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func generate() error {
	filename := "payload.json"
	basePath := "data/valid"
	outputPath := "docs/data/elasticsearch/"

	var checked = map[string]struct{}{}

	for path, mapping := range beater.Routes {

		if path == beater.HealthCheckURL {
			continue
		}
		if mapping.ProcessorFactory == nil {
			continue
		}
		p := mapping.ProcessorFactory()

		data, err := loader.LoadData(filepath.Join(basePath, p.Name(), filename))
		if err != nil {
			return err
		}

		txBatch, tctx, response := beater.ProcessPayload(data, p)
		if response.IsError() {
			return err
		}
		log.Println("!!!!!", tctx.Service)

		events := txBatch.Transform(config.TransformConfig{}, tctx)

		for _, d := range events {
			n, err := d.GetValue("processor.name")
			if err != nil {
				return err
			}
			event, err := d.GetValue("processor.event")
			if err != nil {
				return err
			}

			key := n.(string) + "-" + event.(string)

			if _, ok := checked[key]; !ok {
				checked[key] = struct{}{}
				file := filepath.Join(outputPath, event.(string)+".json")

				output, err := json.MarshalIndent(d.Fields, "", "    ")
				if err != nil {
					return err
				}
				err = ioutil.WriteFile(file, output, 0644)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
