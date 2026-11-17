package logs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var logTime time.Time

func init() {
	logTime, _ = time.Parse(LogTimeFormat, "2023-08-07T19:56:34.223758260-06:00")
}

func makeTestLogLine(typ, msg string) *LogLine {
	return &LogLine{
		Device:       "stdout",
		ParseLogType: typ,
		Msg:          msg,
		Time:         logTime,
	}
}

func TestGetTailLog(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		tail        int
		want        []*LogLine
	}{
		{
			name: "simple tail",
			fileContent: `2023-08-07T19:56:34.223758260-06:00 stdout F line1
2023-08-07T19:56:34.223758260-06:00 stdout F line2
2023-08-07T19:56:34.223758260-06:00 stdout F line3
`,
			tail: 2,
			want: []*LogLine{makeTestLogLine("F", "line2"), makeTestLogLine("F", "line3")},
		},
		{
			name: "simple tail with more tail than lines",
			fileContent: `2023-08-07T19:56:34.223758260-06:00 stdout F line1
2023-08-07T19:56:34.223758260-06:00 stdout F line2
2023-08-07T19:56:34.223758260-06:00 stdout F line3
`,
			tail: 10,
			want: []*LogLine{makeTestLogLine("F", "line1"), makeTestLogLine("F", "line2"), makeTestLogLine("F", "line3")},
		},
		{
			name: "tail with partial logs",
			fileContent: `2023-08-07T19:56:34.223758260-06:00 stdout F line1
2023-08-07T19:56:34.223758260-06:00 stdout P lin
2023-08-07T19:56:34.223758260-06:00 stdout F e2
2023-08-07T19:56:34.223758260-06:00 stdout F line3
`,
			tail: 2,
			want: []*LogLine{makeTestLogLine("P", "lin"), makeTestLogLine("F", "e2"), makeTestLogLine("F", "line3")},
		},
		{
			name: "tail with partial logs and more than lines",
			fileContent: `2023-08-07T19:56:34.223758260-06:00 stdout F line1
2023-08-07T19:56:34.223758260-06:00 stdout P lin
2023-08-07T19:56:34.223758260-06:00 stdout F e2
2023-08-07T19:56:34.223758260-06:00 stdout F line3
`,
			tail: 10,
			want: []*LogLine{makeTestLogLine("F", "line1"), makeTestLogLine("P", "lin"), makeTestLogLine("F", "e2"), makeTestLogLine("F", "line3")},
		},
		{
			name: "multiple partial lines in a row",
			fileContent: `2023-08-07T19:56:34.223758260-06:00 stdout F line1
2023-08-07T19:56:34.223758260-06:00 stdout P l
2023-08-07T19:56:34.223758260-06:00 stdout P i
2023-08-07T19:56:34.223758260-06:00 stdout P n
2023-08-07T19:56:34.223758260-06:00 stdout P e
2023-08-07T19:56:34.223758260-06:00 stdout F 2
2023-08-07T19:56:34.223758260-06:00 stdout F line3
`,
			tail: 2,
			want: []*LogLine{
				makeTestLogLine("P", "l"), makeTestLogLine("P", "i"), makeTestLogLine("P", "n"),
				makeTestLogLine("P", "e"), makeTestLogLine("F", "2"), makeTestLogLine("F", "line3"),
			},
		},
		{
			name: "partial line at the end",
			fileContent: `2023-08-07T19:56:34.223758260-06:00 stdout F line1
2023-08-07T19:56:34.223758260-06:00 stdout P lin
`,
			tail: 1,
			want: []*LogLine{makeTestLogLine("P", "lin")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "log")
			f, err := os.Create(file)
			assert.NoError(t, err, "create log file")
			_, err = f.WriteString(tt.fileContent)
			assert.NoError(t, err, "write log file")
			f.Close()
			got, err := getTailLog(file, tt.tail)
			assert.NoError(t, err, "getTailLog()")
			assert.Equal(t, tt.want, got, "log lines")
		})
	}
}

func TestGetTailLogBigFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "log")
	f, err := os.Create(file)
	assert.NoError(t, err, "create log file")
	want := make([]*LogLine, 0, 2000)
	for range 1000 {
		_, err = f.WriteString(`2023-08-07T19:56:34.223758260-06:00 stdout P lin
2023-08-07T19:56:34.223758260-06:00 stdout F e2
`)
		assert.NoError(t, err, "write log file")
		want = append(want, makeTestLogLine("P", "lin"), makeTestLogLine("F", "e2"))
	}
	f.Close()

	// try a big tail greater than the lines
	got, err := getTailLog(file, 5000)
	assert.NoError(t, err, "getTailLog()")
	assert.Equal(t, want, got, "all log lines")

	// try a smaller than lines tail
	got, err = getTailLog(file, 100)
	assert.NoError(t, err, "getTailLog()")
	// this will return the last 200 lines because of partial + full and we only count full lines for tail.
	assert.Equal(t, want[1800:2000], got, "tail 100 log lines")
}
