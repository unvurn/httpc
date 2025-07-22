package httpc

type Result interface {
	As(any) error
}
