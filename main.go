package main

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/alecthomas/kong"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

var cli struct {
	Directory string `help:"path to local clone of https://github.com/CSSEGISandData/COVID-19." default:"COVID-19"`
	Addr      string `help:"listen address." default:":8080"`
}

func main() {
	kong.Parse(&cli)
	reader := newReader(cli.Directory)

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	http.HandleFunc("/read", func(w http.ResponseWriter, r *http.Request) {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			level.Error(logger).Log("msg", "Read error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			level.Error(logger).Log("msg", "Decode error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.ReadRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			level.Error(logger).Log("msg", "Unmarshal error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var resp *prompb.ReadResponse
		resp, err = reader.Read(&req)
		if err != nil {
			level.Warn(logger).Log("msg", "Error executing query", "query", req, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := proto.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Header().Set("Content-Encoding", "snappy")

		compressed = snappy.Encode(nil, data)
		if _, err := w.Write(compressed); err != nil {
			level.Warn(logger).Log("msg", "Error writing response", "storage", "err", err)
		}
	})

	http.ListenAndServe(cli.Addr, nil)
}
