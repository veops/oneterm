package record

type Record interface {
	Write(record []byte) error
	Close()
	Resize(height, width int) error
}
