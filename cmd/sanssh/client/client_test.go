package client

import (
	"bytes"
	"testing"
)

func TestPrefixWriter(t *testing.T) {

	for _, tc := range []struct {
		desc   string
		inputs []string
		output string
	}{
		{
			desc:   "Basic input",
			inputs: []string{"Hello world\n"},
			output: "prefix: Hello world\n",
		},
		{
			desc:   "Multiple writes for single line",
			inputs: []string{"Hello ", "world\n"},
			output: "prefix: Hello world\n",
		},
		{
			desc:   "Two lines, single write",
			inputs: []string{"Hello\nworld\n"},
			output: "prefix: Hello\nprefix: world\n",
		},
		{
			desc:   "Two lines, multiple writes",
			inputs: []string{"Hello\n", "world\n"},
			output: "prefix: Hello\nprefix: world\n",
		},
		{
			desc:   "Three lines, single write",
			inputs: []string{"Hello\nworld\n\n"},
			output: "prefix: Hello\nprefix: world\nprefix: \n",
		},
		{
			desc:   "Empty input",
			inputs: []string{""},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			writer := &prefixWriter{
				start:  true,
				dest:   &buf,
				prefix: []byte("prefix: "),
			}

			for _, i := range tc.inputs {
				if _, err := writer.Write([]byte(i)); err != nil {
					t.Error(err)
				}
			}
			got := buf.String()
			if got != tc.output {
				t.Errorf("got %q, want %q", got, tc.output)
			}

		})
	}
}
