package main

import (
	"encoding/csv"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/prompb"
)

type reader struct {
	directory string
	series    []prompb.TimeSeries
}

func newReader(directory string) *reader {
	r := &reader{
		directory: directory,
		series:    make([]prompb.TimeSeries, 0),
	}
	r.init()
	return r
}

func (r *reader) init() {
	var files []string

	err := filepath.Walk(filepath.Join(r.directory, "csse_covid_19_data", "csse_covid_19_time_series"), func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, ".csv") {
			r.series = append(r.series, parsefile(file)...)
		}
	}
}

func (r *reader) Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error) {
	resp := prompb.ReadResponse{
		Results: make([]*prompb.QueryResult, 0),
	}

	for i, q := range req.Queries {
		resp.Results = append(resp.Results, &prompb.QueryResult{
			Timeseries: make([]*prompb.TimeSeries, 0),
		})
		for _, ts := range r.series {
			cont := true
			for _, m := range q.Matchers {
				switch m.Type {
				case prompb.LabelMatcher_EQ:
					match := labels.MustNewMatcher(labels.MatchEqual, m.Name, m.Value)
					for _, v := range ts.Labels {
						if v.GetName() == m.Name {
							if !match.Matches(v.GetValue()) {
								cont = false
							}
						}
					}
				case prompb.LabelMatcher_NEQ:
					match := labels.MustNewMatcher(labels.MatchNotEqual, m.Name, m.Value)
					for _, v := range ts.Labels {
						if v.GetName() == m.Name {
							if !match.Matches(v.GetValue()) {
								cont = false
							}
						}
					}
				case prompb.LabelMatcher_RE:
					match := labels.MustNewMatcher(labels.MatchRegexp, m.Name, m.Value)
					for _, v := range ts.Labels {
						if v.GetName() == m.Name {
							if !match.Matches(v.GetValue()) {
								cont = false
							}
						}
					}
				case prompb.LabelMatcher_NRE:
					match := labels.MustNewMatcher(labels.MatchNotRegexp, m.Name, m.Value)
					for _, v := range ts.Labels {
						if v.GetName() == m.Name {
							if !match.Matches(v.GetValue()) {
								cont = false
							}
						}
					}
				default:
					//	return nil, errors.Errorf("unknown match type %v", m.Type)
				}
			}
			if !cont {
				continue
			}
			t := prompb.TimeSeries{
				Labels:  ts.GetLabels(),
				Samples: make([]prompb.Sample, 0),
			}
			for _, s := range ts.GetSamples() {
				if q.GetEndTimestampMs() > s.GetTimestamp() && q.GetStartTimestampMs() < s.GetTimestamp() {
					t.Samples = append(t.Samples, s)
				}
			}
			if len(t.Samples) > 0 {
				resp.Results[i].Timeseries = append(resp.Results[i].Timeseries, &t)
			}
		}
	}
	//	return &resp, nil
	return &resp, nil
}

func parsefile(file string) []prompb.TimeSeries {
	res := make([]prompb.TimeSeries, 0)
	csvfile, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	r := csv.NewReader(csvfile)
	var header []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if header == nil {
			header = record
			continue
		}

		l := make([]prompb.Label, 0)
		d := make([]prompb.Sample, 0)

		l = append(l, prompb.Label{
			Name:  "__name__",
			Value: strings.TrimPrefix(strings.TrimSuffix(path.Base(file), ".csv"), "time_series_"),
		})

		for i, v := range header {
			layout := "1/2/06"
			t, err := time.Parse(layout, v)
			if err != nil {
				if record[i] != "" && v != "Lat" && v != "Long" {
					l = append(l, prompb.Label{
						Name:  strings.ReplaceAll(v, "/", "_"),
						Value: record[i],
					})
				}
				continue
			}
			n, err := strconv.ParseFloat(record[i], 64)
			if err != nil {
				panic(err)
			}
			d = append(d, prompb.Sample{
				Timestamp: 1000 * t.Unix(),
				Value:     n,
			})
		}

		res = append(res, prompb.TimeSeries{
			Labels:  l,
			Samples: d,
		})
	}
	return res
}
