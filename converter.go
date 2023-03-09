package textplain

type Converter interface {
	Convert(string, int) (string, error)
}
