package diff

type diffError string

const (
	ErrDiff diffError = "diff error"
)

func (e diffError) Error() string {
	return string(e)
}
